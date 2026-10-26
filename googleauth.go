package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

// 구글 로그인(OAuth 2.0, 데스크톱 앱 + 루프백 + PKCE)과 토큰 보관.
//
// 클라이언트: 릴리스 빌드는 -ldflags -X로 넣은 기본 클라이언트를 쓴다(.github/workflows/release.yml, 저장소엔 없음).
// 설정 폴더에 google_client.json(Cloud 콘솔에서 받은 데스크톱 앱 클라이언트 JSON)이 있으면 그걸 쓴다.
// 토큰: OS 자격 증명 저장소(맥 키체인·윈도우 자격 증명 관리자·리눅스 Secret Service). 쓸 수 없으면 설정 폴더 파일(0600).

var googleClientID, googleClientSecret string

const (
	gcalScope      = "https://www.googleapis.com/auth/calendar"
	clientFileName = "google_client.json"
	tokenFileName  = "google_token.json" // 자격 증명 저장소를 못 쓸 때만
	keyringService = "calendar-tui"
	keyringUser    = "google"
)

var googleEndpoint = oauth2.Endpoint{
	AuthURL:   "https://accounts.google.com/o/oauth2/auth",
	TokenURL:  "https://oauth2.googleapis.com/token",
	AuthStyle: oauth2.AuthStyleInParams,
}

// oauthConfig는 쓸 OAuth 클라이언트. 설정 폴더의 google_client.json이 기본 클라이언트보다 앞선다.
func oauthConfig() (*oauth2.Config, error) {
	id, secret := googleClientID, googleClientSecret
	path := filepath.Join(configDir(), clientFileName)
	if b, err := os.ReadFile(path); err == nil {
		var f struct {
			Installed struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
			} `json:"installed"`
		}
		if json.Unmarshal(b, &f) != nil || f.Installed.ClientID == "" {
			return nil, fmt.Errorf(L("%s: 데스크톱 앱 OAuth 클라이언트 JSON이 아닙니다", "%s: not a desktop-app OAuth client JSON"), path)
		}
		id, secret = f.Installed.ClientID, f.Installed.ClientSecret
	}
	if id == "" {
		return nil, fmt.Errorf(L("이 빌드에는 구글 OAuth 클라이언트가 없습니다. Cloud 콘솔에서 받은 데스크톱 앱 클라이언트 JSON을 %s에 두세요",
			"This build has no Google OAuth client. Put a desktop-app client JSON from Cloud Console at %s"), path)
	}
	return &oauth2.Config{ClientID: id, ClientSecret: secret, Scopes: []string{gcalScope}, Endpoint: googleEndpoint}, nil
}

// ── 토큰 보관 ──

func tokenFile() string { return filepath.Join(configDir(), tokenFileName) }

func loadToken() (*oauth2.Token, error) {
	s, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		b, ferr := os.ReadFile(tokenFile())
		if ferr != nil {
			return nil, err
		}
		s = string(b)
	}
	var t oauth2.Token
	if err := json.Unmarshal([]byte(s), &t); err != nil || t.RefreshToken == "" {
		return nil, keyring.ErrNotFound
	}
	return &t, nil
}

func saveToken(t *oauth2.Token) error {
	b, _ := json.Marshal(t)
	if keyring.Set(keyringService, keyringUser, string(b)) == nil {
		os.Remove(tokenFile()) // 예전에 파일로 남겼으면 지운다
		return nil
	}
	os.MkdirAll(configDir(), 0o700)
	return os.WriteFile(tokenFile(), b, 0o600)
}

func deleteToken() {
	keyring.Delete(keyringService, keyringUser)
	os.Remove(tokenFile())
}

// savingSource는 토큰이 갱신되면(액세스 토큰은 1시간마다 바뀐다) 보관해 둔다.
type savingSource struct {
	src  oauth2.TokenSource
	mu   sync.Mutex
	last string
}

func (s *savingSource) Token() (*oauth2.Token, error) {
	t, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.AccessToken != s.last {
		s.last = t.AccessToken
		saveToken(t)
	}
	return t, nil
}

// authErr는 토큰 갱신이 거절된 경우(로그인 취소·비밀번호 변경·권한 회수)를 다시 로그인하라는 안내로 바꾼다.
func authErr(err error) error {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) && (re.ErrorCode == "invalid_grant" || re.Response != nil && re.Response.StatusCode == http.StatusUnauthorized) {
		return errors.New(L("구글 로그인이 만료됐습니다. 터미널에서 calendar-tui --login 으로 다시 로그인하세요",
			"Google sign-in expired. Run calendar-tui --login in a terminal"))
	}
	return err
}

// newGoogle은 저장된 로그인으로 구글 백엔드를 만든다. 로그인 정보가 없으면 터미널에서 로그인부터 한다.
func newGoogle() (backend, error) {
	cfg, err := oauthConfig()
	if err != nil {
		return nil, err
	}
	tok, err := loadToken()
	if err != nil {
		if tok, err = googleLogin(cfg); err != nil {
			return nil, err
		}
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: 20 * time.Second})
	hc := oauth2.NewClient(ctx, &savingSource{src: cfg.TokenSource(ctx, tok), last: tok.AccessToken})
	hc.Timeout = 20 * time.Second
	allDayAlarms = googleAllDayAlarms
	return &googleBackend{hc: hc, api: gcalAPI}, nil
}

// googleLogin은 브라우저로 구글 로그인을 받아 토큰을 보관한다(루프백 리디렉션 + PKCE).
// 디바이스 코드 방식은 캘린더 권한을 지원하지 않아 쓰지 못한다.
func googleLogin(cfg *oauth2.Config) (*oauth2.Token, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer ln.Close()
	c := *cfg
	c.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)
	sb := make([]byte, 16)
	rand.Read(sb)
	state, verifier := hex.EncodeToString(sb), oauth2.GenerateVerifier()
	authURL := c.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "consent"))

	type result struct {
		code string
		err  error
	}
	got := make(chan result, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if e := q.Get("error"); e != "" {
			fmt.Fprint(w, loginPage(L("로그인하지 않았습니다. 터미널로 돌아가세요.", "Sign-in cancelled. Return to the terminal.")))
			got <- result{err: fmt.Errorf(L("구글 로그인 취소: %s", "Google sign-in cancelled: %s"), e)}
			return
		}
		fmt.Fprint(w, loginPage(L("로그인했습니다. 이 탭을 닫고 터미널로 돌아가세요.", "Signed in. Close this tab and return to the terminal.")))
		got <- result{code: q.Get("code")}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	fmt.Println(L("구글 캘린더에 로그인합니다. 브라우저가 열리지 않으면 아래 주소를 여세요.",
		"Sign in to Google Calendar. If no browser opens, open this URL:"))
	fmt.Println("\n  " + authURL + "\n")
	fmt.Println(L("\"Google에서 확인하지 않은 앱\" 화면이 나오면 [고급] → [calendar-tui(으)로 이동]을 누르고, 캘린더 권한을 체크한 채 계속하세요.",
		"If you see \"Google hasn't verified this app\", click Advanced → Go to calendar-tui, keep the calendar permission checked and continue."))
	openURL(authURL)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)
	var r result
	select {
	case r = <-got:
	case <-stop:
		return nil, errors.New(L("로그인을 그만뒀습니다", "Sign-in aborted"))
	case <-time.After(10 * time.Minute):
		return nil, errors.New(L("로그인을 10분 동안 기다렸지만 끝나지 않았습니다", "Timed out waiting for sign-in"))
	}
	if r.err != nil {
		return nil, r.err
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: 20 * time.Second})
	tok, err := c.Exchange(ctx, r.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, err
	}
	if sc, _ := tok.Extra("scope").(string); !strings.Contains(sc, gcalScope) {
		return nil, errors.New(L("캘린더 권한을 허용하지 않았습니다. 다시 로그인할 때 캘린더 항목을 체크하세요",
			"Calendar permission was not granted. Sign in again and keep the calendar item checked"))
	}
	if err := saveToken(tok); err != nil {
		return nil, err
	}
	fmt.Println(L("로그인했습니다.", "Signed in."))
	return tok, nil
}

func loginPage(msg string) string {
	return `<!doctype html><meta charset="utf-8"><title>calendar-tui</title>` +
		`<body style="font:16px system-ui;margin:4em;text-align:center">` + msg + `</body>`
}

// googleLogout은 보관한 로그인을 지우고 구글에도 권한 회수를 알린다(실패해도 로컬은 지운다).
func googleLogout() {
	if t, err := loadToken(); err == nil {
		http.PostForm("https://oauth2.googleapis.com/revoke", url.Values{"token": {t.RefreshToken}})
	}
	deleteToken()
}
