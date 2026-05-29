// Package storage implementa el cliente HTTP del protocolo @firma storage/retriever.
// Protocolo: POST application/x-www-form-urlencoded con op=put|get.
package storage

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Put sube datos al storage. Devuelve error si la respuesta no es "OK".
func Put(endpoint, id, dat string) error {
	resp, err := httpClient.PostForm(endpoint, url.Values{
		"op": {"put"}, "v": {"1_0"}, "id": {id}, "dat": {dat},
	})
	if err != nil {
		return fmt.Errorf("PUT %s: %w", id, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != "OK" {
		return fmt.Errorf("PUT %s: status=%d body=%q", id, resp.StatusCode, body)
	}
	return nil
}

// Get recupera datos del storage. Devuelve el body tal cual.
func Get(endpoint, id string) (string, error) {
	resp, err := httpClient.PostForm(endpoint, url.Values{
		"op": {"get"}, "v": {"1_0"}, "id": {id},
	})
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("GET %s: no encontrado (sesión expiró o id inválido)", id)
	}
	body, err := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body)), err
}

// Envelope es el XML que el backend pone en storage para AutoFirmaGDI.
type Envelope struct {
	XMLName xml.Name       `xml:"op"`
	Entries []EnvelopeEntry `xml:"e"`
}

type EnvelopeEntry struct {
	K string `xml:"k,attr"`
	V string `xml:"v,attr"`
}

// FetchEnvelope obtiene el XML del retriever, decodea base64 y lo parsea.
func FetchEnvelope(rtServlet, fileID string) (*Envelope, error) {
	raw, err := Get(rtServlet, fileID)
	if err != nil {
		return nil, err
	}

	// El servidor devuelve el XML en base64 (requerimiento del protocolo @firma).
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("el retriever no devolvió base64 válido: %w", err)
	}

	var env Envelope
	if err := xml.Unmarshal(decoded, &env); err != nil {
		return nil, fmt.Errorf("XML malformado en el envelope: %w", err)
	}
	return &env, nil
}

// Get devuelve el valor de una entrada del envelope por clave, URL-unescapeado.
func (e *Envelope) Get(key string) (string, bool) {
	for _, entry := range e.Entries {
		if entry.K == key {
			v, err := url.QueryUnescape(entry.V)
			if err != nil {
				return entry.V, true
			}
			return v, true
		}
	}
	return "", false
}

// PostResult postea el resultado firmado al stservlet.
// El formato es: base64url-sin-padding(cert_DER + signed_PDF).
func PostResult(stServlet, sessionID string, certDER, signedPDF []byte) error {
	blob := append(certDER, signedPDF...)
	encoded := base64.RawURLEncoding.EncodeToString(blob)
	return Put(stServlet, sessionID, encoded)
}

// PostCancel notifica al backend que el usuario canceló.
func PostCancel(stServlet, sessionID string) error {
	return Put(stServlet, sessionID, "CANCEL")
}
