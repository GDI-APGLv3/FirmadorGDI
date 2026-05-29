// Package signing firma PDFs con PAdES usando una clave PKCS#11.
package signing

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"image/color"
	"image/png"
	"strings"
	"time"

	"github.com/digitorus/pdf"
	"github.com/digitorus/pdfsign/sign"
	"github.com/fogleman/gg"
)

// Appearance define la posición y texto del sello visual.
// Los valores llegan desde el backend vía properties del XML envelope.
type Appearance struct {
	Page         uint32
	LowerLeftX   float64
	LowerLeftY   float64
	UpperRightX  float64
	UpperRightY  float64
	SignerName   string
	SerialNum    string // CUIL/CUIT del firmante (ej: "CUIL 20123456789")
	Seal         string // Cargo/sello del firmante
	Department   string // Departamento/área
	Municipality string // Municipio
	Reason       string
	SignedAt     time.Time
}

// SignPDF firma pdfBytes con el signer dado y devuelve el PDF firmado.
func SignPDF(pdfBytes []byte, cert *x509.Certificate, signer crypto.Signer, app Appearance) ([]byte, error) {
	sealPNG, err := renderSeal(app)
	if err != nil {
		return nil, fmt.Errorf("renderSeal: %w", err)
	}

	in := bytes.NewReader(pdfBytes)
	var out bytes.Buffer

	rdr, err := pdf.NewReader(in, int64(len(pdfBytes)))
	if err != nil {
		return nil, fmt.Errorf("pdf.NewReader: %w", err)
	}

	signData := sign.SignData{
		Signature: sign.SignDataSignature{
			Info: sign.SignDataSignatureInfo{
				Name:     app.SignerName,
				Location: "Buenos Aires, Argentina",
				Reason:   app.Reason,
				Date:     app.SignedAt,
			},
			CertType:   sign.ApprovalSignature,
			DocMDPPerm: sign.AllowFillingExistingFormFieldsAndSignaturesPerms,
		},
		Signer:            signer,
		DigestAlgorithm:   crypto.SHA256,
		Certificate:       cert,
		CertificateChains: [][]*x509.Certificate{{cert}},
		Appearance: sign.Appearance{
			Visible:          true,
			Page:             app.Page,
			LowerLeftX:       app.LowerLeftX,
			LowerLeftY:       app.LowerLeftY,
			UpperRightX:      app.UpperRightX,
			UpperRightY:      app.UpperRightY,
			Image:            sealPNG,
			ImageAsWatermark: false,
		},
	}

	if err := sign.Sign(in, &out, rdr, int64(len(pdfBytes)), signData); err != nil {
		return nil, fmt.Errorf("sign.Sign: %w", err)
	}
	return out.Bytes(), nil
}

// AppearanceFromProperties parsea el string Java Properties (base64) que manda el backend.
// Formato: base64(signaturePage=last\nsignaturePositionOnPageLowerLeftX=50\n...layer2Text=$$SUBJECTCN$$\nSello\nDepto\nMunicipio)
func AppearanceFromProperties(propsB64 string, signerName, serialNum string) (*Appearance, error) {
	decoded, err := base64.StdEncoding.DecodeString(propsB64)
	if err != nil {
		return nil, fmt.Errorf("properties no es base64 válido: %w", err)
	}

	props := parseJavaProperties(string(decoded))
	app := &Appearance{
		Page:        uint32Prop(props, "signaturePage", 1),
		LowerLeftX:  floatProp(props, "signaturePositionOnPageLowerLeftX", 365),
		LowerLeftY:  floatProp(props, "signaturePositionOnPageLowerLeftY", 30),
		UpperRightX: floatProp(props, "signaturePositionOnPageUpperRightX", 565),
		UpperRightY: floatProp(props, "signaturePositionOnPageUpperRightY", 90),
		Reason:      props["signReason"],
		SignerName:  signerName,
		SerialNum:   serialNum,
		SignedAt:    time.Now().Local(),
	}

	// Parsear layer2Text: "NombreCompleto\nSello\nDepartamento\nMunicipio"
	// El backend usa \n literal (backslash-n) como separador.
	if layer2 := props["layer2Text"]; layer2 != "" {
		lines := strings.Split(layer2, `\n`)
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			switch i {
			case 0:
				// Nombre del sistema (full_name de BD). Si es el placeholder del cert, conservar signerName.
				if line != "$$SUBJECTCN$$" {
					app.SignerName = line
				}
			case 1:
				app.Seal = line
			case 2:
				app.Department = line
			case 3:
				app.Municipality = line
			}
		}
	}

	return app, nil
}

// renderSeal genera el PNG del sello visual idéntico a la firma electrónica GDI (pyHanko).
// La firma electrónica usa: Courier 10pt negro, caja 200x80pt, 4 líneas pegadas arriba,
// nombre en MAYÚSCULAS, resto en case normal. Sin CUIL, sin fecha, sin borde.
//
// El PNG se estira a la caja (LowerLeft..UpperRight = 200x80pt), así que mantenemos
// ese mismo aspect ratio (2.5:1) y un factor de escala 5x para nitidez (1000x400px).
func renderSeal(app Appearance) ([]byte, error) {
	const (
		scale   = 5.0
		boxW    = 200.0 // SIGNATURE_WIDTH de Notary
		boxH    = 80.0  // SIGNATURE_HEIGHT de Notary
		w       = int(boxW * scale)
		h       = int(boxH * scale)
		fontPt  = 10.0          // Courier 10pt (igual que pyHanko)
		fontPx  = fontPt * scale
		leftPad = 2.0 * scale   // padding izquierdo en px
		lineH   = fontPt * scale // interlineado = tamaño de fuente (líneas pegadas)
	)

	dc := gg.NewContext(w, h)
	dc.SetColor(color.White)
	dc.Clear()
	dc.SetColor(color.Black)

	courier := "C:/Windows/Fonts/cour.ttf"
	if err := dc.LoadFontFace(courier, fontPx); err != nil {
		// Fallback: Consolas, también monospace en Windows
		if err2 := dc.LoadFontFace("C:/Windows/Fonts/consola.ttf", fontPx); err2 != nil {
			return nil, fmt.Errorf("no se pudo cargar fuente monospace: %w", err)
		}
	}

	// 4 líneas: nombre (mayúsculas) + sello + departamento + municipio
	lines := []string{strings.ToUpper(app.SignerName), app.Seal, app.Department, app.Municipality}

	// Baseline de la primera línea cerca del borde superior (texto pegado arriba como pyHanko)
	y := fontPx * 0.85
	for _, line := range lines {
		if line != "" {
			dc.DrawString(line, leftPad, y)
		}
		y += lineH
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseJavaProperties(s string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

func uint32Prop(m map[string]string, k string, def uint32) uint32 {
	v, ok := m[k]
	if !ok {
		return def
	}
	var n uint32
	fmt.Sscanf(v, "%d", &n)
	return n
}

func intProp(m map[string]string, k string, def int) int {
	v, ok := m[k]
	if !ok {
		return def
	}
	var n int
	fmt.Sscanf(v, "%d", &n)
	return n
}

func floatProp(m map[string]string, k string, def float64) float64 {
	v, ok := m[k]
	if !ok {
		return def
	}
	var f float64
	fmt.Sscanf(v, "%f", &f)
	return f
}
