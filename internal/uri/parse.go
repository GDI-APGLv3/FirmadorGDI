// Package uri parsea y valida URIs del scheme gdifirma://.
// Protocolo compatible con @firma 1.9 (misma estructura que afirma://sign?...).
package uri

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const Scheme = "gdifirma"

var alphanumRE = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// Params contiene los parámetros extraídos de la URI gdifirma://sign?...
type Params struct {
	Ver        string
	FileID     string // id del XML en storage (rtservlet)
	RtServlet  string // URL donde buscar el XML
	StServlet  string // URL donde postear la firma
	SessionID  string // id de la sesión de firma
	Keystore   string // PKCS11 | WINDOWS | MAC
}

// Parse extrae los parámetros de una URI gdifirma://.
// Chrome puede agregar una / después del host (gdifirma://sign/ en lugar de gdifirma://sign).
func Parse(raw string) (*Params, error) {
	if !strings.HasPrefix(raw, Scheme+"://") {
		return nil, fmt.Errorf("scheme inválido, se esperaba %s://", Scheme)
	}

	// Normalizar: Chrome convierte gdifirma://sign?... en gdifirma://sign/?...
	// Reemplazamos el scheme para que url.Parse lo acepte como http.
	normalized := "https://" + strings.TrimPrefix(raw, Scheme+"://")
	u, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("URI malformada: %w", err)
	}

	q := u.Query()
	p := &Params{
		Ver:       q.Get("ver"),
		FileID:    q.Get("fileid"),
		RtServlet: q.Get("rtservlet"),
		StServlet: q.Get("stservlet"),
		SessionID: q.Get("id"),
		Keystore:  q.Get("keystore"),
	}

	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// isAllowedServletURL permite HTTPS en producción y HTTP localhost para pruebas locales.
func isAllowedServletURL(u string) bool {
	return strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "http://localhost") ||
		strings.HasPrefix(u, "http://127.0.0.1")
}

func (p *Params) validate() error {
	if p.FileID == "" {
		return fmt.Errorf("falta parámetro 'fileid'")
	}
	if !alphanumRE.MatchString(p.FileID) {
		return fmt.Errorf("fileid debe ser alfanumérico puro (sin guiones ni puntos): %q", p.FileID)
	}
	if p.SessionID == "" {
		return fmt.Errorf("falta parámetro 'id'")
	}
	if !alphanumRE.MatchString(p.SessionID) {
		return fmt.Errorf("id debe ser alfanumérico puro: %q", p.SessionID)
	}
	if p.RtServlet == "" {
		return fmt.Errorf("falta parámetro 'rtservlet'")
	}
	if !isAllowedServletURL(p.RtServlet) {
		return fmt.Errorf("rtservlet debe ser HTTPS (o localhost para pruebas): %q", p.RtServlet)
	}
	if p.StServlet == "" {
		return fmt.Errorf("falta parámetro 'stservlet'")
	}
	if !isAllowedServletURL(p.StServlet) {
		return fmt.Errorf("stservlet debe ser HTTPS (o localhost para pruebas): %q", p.StServlet)
	}
	return nil
}
