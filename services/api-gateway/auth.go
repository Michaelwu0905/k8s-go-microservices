package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"mime"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/driver_client"
	driverpb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
	"golang.org/x/crypto/bcrypt"
)

//go:embed auth_schema.sql
var authSchema string

const sessionCookie = "ride_session"
const sessionLifetime = 7 * 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

type Principal struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CSRFToken   string    `json:"csrf_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	tokenHash   []byte
}
type principalKey struct{}

func currentUser(r *http.Request) *Principal {
	user, _ := r.Context().Value(principalKey{}).(*Principal)
	return user
}
func authError(w http.ResponseWriter, code int, message string) {
	_ = writeJSON(w, code, map[string]string{"error": message})
}

type AuthStore struct {
	pool          *pgxpool.Pool
	dummyHash     []byte
	secureCookies bool
}

func NewAuthStore(ctx context.Context, databaseURL string) (*AuthStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(734893203)"); err == nil {
		_, err = tx.Exec(ctx, authSchema)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		pool.Close()
		return nil, err
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-comparison-not-a-login-account"), bcrypt.DefaultCost)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &AuthStore{pool: pool, dummyHash: dummy, secureCookies: os.Getenv("SESSION_COOKIE_SECURE") == "true"}, nil
}
func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func tokenDigest(token string) []byte { hash := sha256.Sum256([]byte(token)); return hash[:] }
func csrfToken(token string) string {
	hash := sha256.Sum256([]byte("ride-sharing-csrf:" + token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
func (a *AuthStore) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(sessionLifetime.Seconds())})
}
func (a *AuthStore) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func (a *AuthStore) createSession(ctx context.Context, w http.ResponseWriter, user *Principal) error {
	token, err := randomToken()
	if err != nil {
		return err
	}
	user.ExpiresAt = time.Now().UTC().Add(sessionLifetime)
	user.CSRFToken = csrfToken(token)
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM auth_sessions WHERE expires_at<now()"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO auth_sessions(token_hash,account_id,expires_at) VALUES($1,$2,$3)", tokenDigest(token), user.ID, user.ExpiresAt); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	a.setCookie(w, token, user.ExpiresAt)
	return nil
}
func (a *AuthStore) authenticate(ctx context.Context, r *http.Request) (*Principal, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, pgx.ErrNoRows
	}
	token, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(token) != 32 {
		return nil, pgx.ErrNoRows
	}
	user := &Principal{tokenHash: tokenDigest(cookie.Value)}
	err = a.pool.QueryRow(ctx, `SELECT a.id,a.username,a.display_name,a.role,s.expires_at FROM auth_sessions s
 JOIN accounts a ON a.id=s.account_id WHERE s.token_hash=$1 AND s.expires_at>now()`, user.tokenHash).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.ExpiresAt)
	if err != nil {
		return nil, err
	}
	user.CSRFToken = csrfToken(cookie.Value)
	return user, nil
}
func (a *AuthStore) sessionValid(ctx context.Context, hash []byte) bool {
	var valid bool
	err := a.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE token_hash=$1 AND expires_at>now())", hash).Scan(&valid)
	return err == nil && valid
}
func (a *AuthStore) middleware(next http.Handler) http.Handler {
	return enableCORS(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/auth/register" || r.URL.Path == "/auth/login" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		user, err := a.authenticate(ctx, r)
		cancel()
		if errors.Is(err, pgx.ErrNoRows) {
			a.clearCookie(w)
			authError(w, 401, "请先登录或重新登录")
			return
		}
		if err != nil {
			authError(w, 503, "登录服务暂不可用")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(user.CSRFToken)) != 1 {
				authError(w, 403, "请求校验失败，请刷新后重试")
				return
			}
		}
		ctx, cancel = context.WithDeadline(r.Context(), user.ExpiresAt)
		defer cancel()
		ctx = context.WithValue(ctx, principalKey{}, user)
		if r.URL.Path == "/events/trips" || r.URL.Path == "/ws/drivers" {
			go func() {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						checkCtx, done := context.WithTimeout(ctx, time.Second)
						valid := a.sessionValid(checkCtx, user.tokenHash)
						done()
						if !valid {
							cancel()
							return
						}
					}
				}
			}()
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (a *AuthStore) allowAttempt(ctx context.Context, r *http.Request, kind, username string) (bool, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Ignore untrusted forwarding headers; use the actual peer address.
	key := hex.EncodeToString(tokenDigest(kind + ":" + ip + ":" + username))
	var attempts int
	err = a.pool.QueryRow(ctx, `INSERT INTO auth_rate_limits(key,attempts,expires_at) VALUES($1,1,now()+interval '15 minutes')
 ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN auth_rate_limits.expires_at<now() THEN 1 ELSE auth_rate_limits.attempts+1 END,
 expires_at=CASE WHEN auth_rate_limits.expires_at<now() THEN now()+interval '15 minutes' ELSE auth_rate_limits.expires_at END RETURNING attempts`, key).Scan(&attempts)
	limit := 20
	if kind == "ip" {
		limit = 100
	}
	return attempts <= limit, err
}
func (a *AuthStore) registerRoutes(mux *http.ServeMux, drivers *driver_client.DriverServiceClient) {
	credentials := func(w http.ResponseWriter, r *http.Request, register bool) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			authError(w, 405, "仅支持 POST")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			authError(w, 415, "请使用 application/json")
			return
		}
		var body struct {
			Username    string `json:"username"`
			Password    string `json:"password"`
			DisplayName string `json:"display_name"`
			Role        string `json:"role"`
		}
		if !decodeTripBody(w, r, &body) {
			return
		}
		body.Username = strings.ToLower(strings.TrimSpace(body.Username))
		body.DisplayName = strings.TrimSpace(body.DisplayName)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if len(body.Password) > 72 {
			authError(w, 400, "密码最多 72 字节")
			return
		}
		for _, kind := range []string{"ip", "credentials"} {
			name := body.Username
			if kind == "ip" {
				name = ""
			}
			allowed, err := a.allowAttempt(ctx, r, kind, name)
			if err != nil {
				authError(w, 503, "登录服务暂不可用")
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "900")
				authError(w, 429, "尝试过于频繁，请稍后重试")
				return
			}
		}
		user := &Principal{}
		if register {
			if !usernamePattern.MatchString(body.Username) || len(body.Password) < 10 || utf8.RuneCountInString(body.DisplayName) < 1 || utf8.RuneCountInString(body.DisplayName) > 40 || (body.Role != "rider" && body.Role != "driver") {
				authError(w, 400, "账号需为 3–32 位字母、数字或下划线；密码至少 10 字节；昵称 1–40 字；请选择乘客或司机身份")
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
			if err != nil {
				authError(w, 500, "无法创建账号")
				return
			}
			id, err := randomToken()
			if err != nil {
				authError(w, 500, "无法创建账号")
				return
			}
			user = &Principal{ID: body.Role + "-" + id, Username: body.Username, DisplayName: body.DisplayName, Role: body.Role}
			_, err = a.pool.Exec(ctx, "INSERT INTO accounts(id,username,display_name,role,password_hash) VALUES($1,$2,$3,$4,$5)", user.ID, user.Username, user.DisplayName, user.Role, hash)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				authError(w, 409, "该账号已被使用")
				return
			}
			if err != nil {
				authError(w, 503, "注册服务暂不可用")
				return
			}
		} else {
			var hash []byte
			err = a.pool.QueryRow(ctx, "SELECT id,username,display_name,role,password_hash FROM accounts WHERE username=$1", body.Username).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &hash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				authError(w, 503, "登录服务暂不可用")
				return
			}
			if len(hash) == 0 {
				hash = a.dummyHash
			}
			passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(body.Password))
			if err != nil || passwordErr != nil {
				authError(w, 401, "账号或密码错误")
				return
			}
		}
		if err := a.createSession(ctx, w, user); err != nil {
			authError(w, 503, "无法建立登录会话，请重新登录")
			return
		}
		code := http.StatusOK
		if register {
			code = http.StatusCreated
		}
		_ = writeJSON(w, code, user)
	}
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) { credentials(w, r, true) })
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) { credentials(w, r, false) })
	mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			authError(w, 405, "仅支持 GET")
			return
		}
		_ = writeJSON(w, 200, currentUser(r))
	})
	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			authError(w, 405, "仅支持 POST")
			return
		}
		user := currentUser(r)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, err := a.pool.Exec(ctx, "DELETE FROM auth_sessions WHERE token_hash=$1", user.tokenHash); err != nil {
			authError(w, 503, "退出失败，请重试")
			return
		}
		if user.Role == "driver" && drivers != nil {
			_, _ = drivers.Client.SetAvailability(ctx, &driverpb.SetAvailabilityRequest{DriverId: user.ID, Online: false})
		}
		a.clearCookie(w)
		_ = writeJSON(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := a.pool.Ping(ctx); err != nil {
			authError(w, 503, "Database unavailable")
			return
		}
		_ = writeJSON(w, 200, map[string]string{"status": "ok"})
	})
}
