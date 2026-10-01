package iam

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/errs"
	"github.com/blinge12/efoy/pkg/httpx"
)

// Handler exposes the iam endpoints of api/openapi/efoy.yaml under /v1.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts the handlers. The router must already run authz.Authenticate.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/auth/otp/request", h.requestOTP)
	r.Post("/auth/otp/verify", h.verifyOTP)
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.With(authz.RequireAuth).Post("/auth/logout", h.logout)
	r.With(authz.RequireAuth).Get("/me", h.me)
	r.With(authz.RequireAuth).Patch("/me", h.updateMe)
}

// Request and response bodies (schemas in efoy.yaml).

type otpRequestBody struct {
	PhoneE164 string `json:"phone_e164"`
}

type otpRequestedBody struct {
	ExpiresInS   int `json:"expires_in_s"`
	ResendAfterS int `json:"resend_after_s"`
}

type deviceBody struct {
	Platform   string  `json:"platform"`
	PushToken  *string `json:"push_token"`
	AppVersion *string `json:"app_version"`
	OSVersion  *string `json:"os_version"`
}

type otpVerifyBody struct {
	PhoneE164 string      `json:"phone_e164"`
	Code      string      `json:"code"`
	Device    *deviceBody `json:"device"`
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

type roleGrantBody struct {
	Role    string  `json:"role"`
	Scope   string  `json:"scope"`
	ScopeID *string `json:"scope_id"`
}

type meBody struct {
	ID                string          `json:"id"`
	FullName          string          `json:"full_name"`
	FullNameAm        *string         `json:"full_name_am"`
	PhoneE164         *string         `json:"phone_e164"`
	Email             *string         `json:"email"`
	PreferredLanguage string          `json:"preferred_language"`
	PhotoURL          *string         `json:"photo_url"`
	Roles             []roleGrantBody `json:"roles"`
	LinkedRiderIDs    []string        `json:"linked_rider_ids"`
}

type tokenPairBody struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	User         meBody `json:"user"`
}

func toMeBody(p Profile) meBody {
	roles := make([]roleGrantBody, 0, len(p.Grants))
	for _, g := range p.Grants {
		rg := roleGrantBody{Role: string(g.Role), Scope: string(g.Scope)}
		if g.Scope != authz.ScopeGlobal {
			id := g.ScopeID.String()
			rg.ScopeID = &id
		}
		roles = append(roles, rg)
	}
	riders := make([]string, 0, len(p.LinkedRiderIDs))
	for _, id := range p.LinkedRiderIDs {
		riders = append(riders, id.String())
	}
	return meBody{
		ID:                p.User.ID.String(),
		FullName:          p.User.FullName,
		FullNameAm:        p.User.FullNameAm,
		PhoneE164:         p.User.Phone,
		Email:             p.User.Email,
		PreferredLanguage: p.User.PreferredLanguage,
		PhotoURL:          nil, // signed MinIO URLs arrive with document uploads (day 3)
		Roles:             roles,
		LinkedRiderIDs:    riders,
	}
}

func toTokenPairBody(p TokenPair) tokenPairBody {
	return tokenPairBody{
		AccessToken:  p.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(p.ExpiresIn.Seconds()),
		RefreshToken: p.RefreshToken,
		User:         toMeBody(p.Profile),
	}
}

var errInvalidJSON = errs.Invalid("INVALID_JSON", "The request body must be valid JSON.")

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errInvalidJSON
	}
	return nil
}

// clientMeta reads the caller's IP (after chi's RealIP middleware) and agent.
func clientMeta(r *http.Request) ClientMeta {
	ip := ""
	if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		ip = ap.Addr().String()
	} else if a, err := netip.ParseAddr(r.RemoteAddr); err == nil {
		ip = a.String()
	}
	ua := r.UserAgent()
	if len(ua) > 512 {
		ua = ua[:512]
	}
	return ClientMeta{IP: ip, UserAgent: ua}
}

// language picks am, en or om from Accept-Language (default am).
func language(r *http.Request) string {
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")
		switch base {
		case "am", "en", "om":
			return base
		}
	}
	return "am"
}

func (h *Handler) requestOTP(w http.ResponseWriter, r *http.Request) {
	var body otpRequestBody
	if err := decode(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	res, err := h.svc.RequestOTP(r.Context(), body.PhoneE164, language(r))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, otpRequestedBody{
		ExpiresInS:   int(res.ExpiresIn.Seconds()),
		ResendAfterS: int(res.ResendAfter.Seconds()),
	})
}

func (h *Handler) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var body otpVerifyBody
	if err := decode(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	var device *DeviceInput
	if body.Device != nil {
		device = &DeviceInput{
			Platform:   body.Device.Platform,
			PushToken:  body.Device.PushToken,
			AppVersion: body.Device.AppVersion,
			OSVersion:  body.Device.OSVersion,
		}
	}
	pair, err := h.svc.VerifyOTP(r.Context(), body.PhoneE164, body.Code, device, clientMeta(r))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	writeTokens(w, pair)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := decode(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	pair, err := h.svc.Login(r.Context(), body.Email, body.Password, body.TOTPCode, clientMeta(r))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	writeTokens(w, pair)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := decode(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	pair, err := h.svc.Refresh(r.Context(), body.RefreshToken, clientMeta(r))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	writeTokens(w, pair)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	actor, err := authz.Require(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := h.svc.Logout(r.Context(), actor); err != nil {
		errs.Write(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	actor, err := authz.Require(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	profile, err := h.svc.Me(r.Context(), actor)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toMeBody(profile))
}

// writeTokens sends a token pair. Token responses must never be cached.
func writeTokens(w http.ResponseWriter, pair TokenPair) {
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, http.StatusOK, toTokenPairBody(pair))
}

// meUpdateBody distinguishes a missing full_name_am (keep) from null (clear).
type meUpdateBody struct {
	FullName          *string         `json:"full_name"`
	FullNameAm        json.RawMessage `json:"full_name_am"`
	PreferredLanguage *string         `json:"preferred_language"`
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	actor, err := authz.Require(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	var body meUpdateBody
	if err := decode(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	in := ProfileUpdate{FullName: body.FullName, PreferredLanguage: body.PreferredLanguage}
	switch {
	case len(body.FullNameAm) == 0:
	case string(body.FullNameAm) == "null":
		in.ClearFullNameAm = true
	default:
		var am string
		if err := json.Unmarshal(body.FullNameAm, &am); err != nil {
			errs.Write(w, r, errInvalidJSON)
			return
		}
		in.FullNameAm = &am
	}
	profile, err := h.svc.UpdateProfile(r.Context(), actor, in)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toMeBody(profile))
}
