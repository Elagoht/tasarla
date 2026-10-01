// Package config reads the application's configuration from the environment.
package config

import (
	"encoding/hex"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Locales are the locales the application is translated into.
var Locales = []string{"tr", "en"}

// OIDC is the identity provider the application signs readers in with.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string // registered with the provider
	CallbackPath string // RedirectURL's path, where the callback is served
	Scopes       []string
}

// Config is everything the process reads from its environment.
type Config struct {
	BaseURL       string
	DatabaseURL   string
	DefaultLocale string
	OIDC          OIDC
	AdminEmails   []string
	SessionKey    []byte
	CSRFKey       []byte
	FlashKey      []byte

	// Location is the time zone "today" and schedules are reckoned in.
	Location       *time.Location
	AttachmentsDir string
	SMTP           SMTP
}

// SMTP is the server e-mails are sent through.
type SMTP struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// Load reads the configuration through getenv and reports every missing or
// malformed value by name, not only the first.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	fail := func(name, message string) {
		errs = append(errs, errors.New(name+": "+message))
	}
	required := func(name string) string {
		v := strings.TrimSpace(getenv(name))
		if v == "" {
			fail(name, "is required")
		}
		return v
	}
	key := func(name string) []byte {
		v := required(name)
		if v == "" {
			return nil
		}
		b, err := hex.DecodeString(v)
		if err != nil || len(b) < 32 {
			fail(name, "must be at least 32 bytes, hex-encoded (openssl rand -hex 32)")
			return nil
		}
		return b
	}

	var cfg Config

	if base := required("BASE_URL"); base != "" {
		u, err := url.Parse(base)
		switch {
		case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
			fail("BASE_URL", "must be an absolute http or https URL")
		case strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "":
			fail("BASE_URL", "must be an origin with no path, query or fragment")
		default:
			cfg.BaseURL = u.Scheme + "://" + u.Host
		}
	}

	cfg.DatabaseURL = required("DATABASE_URL")

	cfg.DefaultLocale = strings.TrimSpace(getenv("DEFAULT_LOCALE"))
	if cfg.DefaultLocale == "" {
		cfg.DefaultLocale = "tr"
	}
	if !slices.Contains(Locales, cfg.DefaultLocale) {
		fail("DEFAULT_LOCALE", "must be one of "+strings.Join(Locales, ", "))
	}

	if issuer := required("OIDC_ISSUER"); issuer != "" {
		u, err := url.Parse(issuer)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail("OIDC_ISSUER", "must be an absolute http or https URL")
		}
		cfg.OIDC.Issuer = issuer
	}
	cfg.OIDC.ClientID = required("OIDC_CLIENT_ID")
	cfg.OIDC.ClientSecret = required("OIDC_CLIENT_SECRET")
	if redirect := required("OIDC_REDIRECT_URL"); redirect != "" {
		u, err := url.Parse(redirect)
		switch {
		case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
			fail("OIDC_REDIRECT_URL", "must be an absolute http or https URL")
		case cfg.BaseURL != "" && u.Scheme+"://"+u.Host != cfg.BaseURL:
			fail("OIDC_REDIRECT_URL", "must be on BASE_URL ("+cfg.BaseURL+")")
		case strings.Trim(u.Path, "/") == "" || u.RawQuery != "" || u.Fragment != "":
			fail("OIDC_REDIRECT_URL", "must name a path, with no query or fragment")
		default:
			cfg.OIDC.RedirectURL = redirect
			cfg.OIDC.CallbackPath = u.Path
		}
	}
	cfg.OIDC.Scopes = strings.Fields(getenv("OIDC_SCOPES"))
	if len(cfg.OIDC.Scopes) == 0 {
		cfg.OIDC.Scopes = []string{"openid", "email", "profile"}
	}
	if !slices.Contains(cfg.OIDC.Scopes, "openid") {
		fail("OIDC_SCOPES", "must include openid")
	}

	for _, e := range strings.Split(getenv("ADMIN_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			cfg.AdminEmails = append(cfg.AdminEmails, e)
		}
	}

	cfg.Location = time.Local
	if tz := strings.TrimSpace(getenv("TIMEZONE")); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			fail("TIMEZONE", "must be an IANA time zone name, such as Europe/Istanbul")
		} else {
			cfg.Location = loc
		}
	}

	cfg.SessionKey = key("SESSION_KEY")
	cfg.CSRFKey = key("CSRF_KEY")
	cfg.FlashKey = key("FLASH_KEY")
	cfg.AttachmentsDir = required("ATTACHMENTS_DIR")
	cfg.SMTP = SMTP{
		Host:     required("SMTP_HOST"),
		Port:     strings.TrimSpace(getenv("SMTP_PORT")),
		User:     strings.TrimSpace(getenv("SMTP_USER")),
		Password: getenv("SMTP_PASSWORD"),
		From:     required("SMTP_FROM"),
	}
	if cfg.SMTP.Port == "" {
		cfg.SMTP.Port = "587"
	}
	if n, err := strconv.Atoi(cfg.SMTP.Port); err != nil || n <= 0 || n > 65535 {
		fail("SMTP_PORT", "must be a port number")
	}

	return cfg, errors.Join(errs...)
}
