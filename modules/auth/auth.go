package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi"
	authbase "github.com/gocontrib/auth"
	"github.com/gocontrib/auth/oauth"
	"github.com/markbates/goth/providers/vk"
)

var (
	authConfig  = makeAuthConfig()
	requireUser = authbase.RequireUser(authConfig)
)

func AuthAPI(mux chi.Router) {
	mux.Post("/api/login", authbase.LoginHandlerFunc(authConfig))
	mux.Post("/api/register", authbase.RegisterHandlerFunc(authConfig))

	oauth.WithProviders(authConfig, "vk", vk.New)
	oauth.RegisterAPI(mux, authConfig)
}

func makeAuthConfig() *authbase.Config {
	userStore := makeUserStore()
	return &authbase.Config{
		UserStore: userStore,
		UserStoreEx: userStore,
		ServerURL: os.Getenv("SERVER_URL"),
	}
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		claims := extractClaims(r)
		userID := get(claims, "user_id")
		userName := get(claims, "user_name")
		email := get(claims, "email")
		role := get(claims, "role")

		if len(userID) > 0 && len(userName) > 0 {
			var user authbase.User = &authbase.UserInfo{
				ID:    userID,
				Name:  userName,
				Email: email,
				Admin: true,
				Claims: map[string]interface{}{
					"email": email,
					"role":  role,
				},
			}

			ctx := r.Context()

			if userID != "system" {
				user, err = authConfig.UserStore.FindUserByID(ctx, userID)
				if err != nil {
					authbase.SendError(w, authbase.ErrUserNotFound.WithCause(err))
					return
				}
			}

			ctx = authbase.WithUser(ctx, user)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
			return
		}

		requireUser(next).ServeHTTP(w, r)
	})
}

func extractClaims(r *http.Request) map[string]string {
	result := make(map[string]string)
	prefix := "Token-Claim-"
	for k, v := range r.Header {
		if strings.HasPrefix(k, prefix) {
			name := strings.ToLower(k[len(prefix):])
			result[name] = v[0]
		}
	}
	return result
}

func get(m map[string]string, k string) string {
	s, ok := m[k]
	if ok {
		return s
	}
	return ""
}
