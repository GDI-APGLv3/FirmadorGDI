// testserver — servidor local que simula rtservlet + stservlet del protocolo @firma.
// Uso: go run ./cmd/testserver  → imprime la URL gdifirma:// para abrir en Chrome.
package main

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	port      = "8765"
	fileID    = "TESTFILE001"
	sessionID = "TESTSESSION001"
)

// minimalPDF genera un PDF de una página válido con offsets calculados al vuelo.
func minimalPDF() []byte {
	header := "%PDF-1.4\n"
	obj1 := "1 0 obj\n<</Type/Catalog/Pages 2 0 R>>\nendobj\n"
	obj2 := "2 0 obj\n<</Type/Pages/Kids[3 0 R]/Count 1>>\nendobj\n"
	stream := "BT\n/F1 14 Tf\n72 700 Td\n(Documento de prueba - FirmadorGDI) Tj\nET\n"
	obj3 := fmt.Sprintf("3 0 obj\n<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>\nendobj\n")
	obj4 := fmt.Sprintf("4 0 obj\n<</Length %d>>\nstream\n%sendstream\nendobj\n", len(stream), stream)
	obj5 := "5 0 obj\n<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>\nendobj\n"

	off1 := len(header)
	off2 := off1 + len(obj1)
	off3 := off2 + len(obj2)
	off4 := off3 + len(obj3)
	off5 := off4 + len(obj4)
	xrefPos := off5 + len(obj5)

	xref := fmt.Sprintf(
		"xref\n0 6\n0000000000 65535 f \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \ntrailer\n<</Size 6/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n",
		off1, off2, off3, off4, off5, xrefPos,
	)

	return []byte(header + obj1 + obj2 + obj3 + obj4 + obj5 + xref)
}

type envelope struct {
	XMLName xml.Name        `xml:"op"`
	Entries []envelopeEntry `xml:"e"`
}

type envelopeEntry struct {
	K string `xml:"k,attr"`
	V string `xml:"v,attr"`
}

var signedResult []byte

func main() {
	// Si se pasa un PDF como argumento, usarlo en lugar del generado.
	pdf := minimalPDF()
	if len(os.Args) > 1 {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			log.Fatalf("No se pudo leer el PDF: %v", err)
		}
		pdf = data
		fmt.Printf("Usando PDF: %s (%d bytes)\n", os.Args[1], len(pdf))
	} else {
		fmt.Printf("Usando PDF de prueba embebido (%d bytes)\n", len(pdf))
	}

	// Preparar el envelope XML con el PDF en base64.
	pdfB64 := url.QueryEscape(base64.StdEncoding.EncodeToString(pdf))
	env := envelope{
		Entries: []envelopeEntry{
			{K: "dat", V: pdfB64},
			{K: "op", V: "sign"},
			{K: "format", V: "PADES"},
		},
	}
	envXML, _ := xml.Marshal(env)
	envB64 := base64.StdEncoding.EncodeToString(envXML)

	// rtservlet: devuelve el envelope.
	http.HandleFunc("/rt", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		op := r.FormValue("op")
		id := r.FormValue("id")
		log.Printf("rtservlet: op=%s id=%s", op, id)
		if op == "get" {
			fmt.Fprint(w, envB64)
		} else {
			http.Error(w, "op desconocida", 400)
		}
	})

	// stservlet: recibe el resultado firmado y lo guarda.
	http.HandleFunc("/st", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		op := r.FormValue("op")
		id := r.FormValue("id")
		dat := r.FormValue("dat")
		log.Printf("stservlet: op=%s id=%s datLen=%d", op, id, len(dat))

		if op == "put" {
			if dat == "CANCEL" {
				fmt.Println("\n⚠  El usuario canceló la firma.")
				fmt.Fprint(w, "OK")
				return
			}
			// Decodear blob: cert_DER + signed_PDF.
			blob, err := base64.RawURLEncoding.DecodeString(dat)
			if err != nil {
				log.Printf("Error decodificando resultado: %v", err)
				http.Error(w, "base64 inválido", 400)
				return
			}
			// Guardar el PDF firmado (heurística: PDF empieza con %PDF-).
			outPath, _ := filepath.Abs("test_signed.pdf")
			if idx := strings.Index(string(blob), "%PDF-"); idx >= 0 {
				if err := os.WriteFile(outPath, blob[idx:], 0644); err != nil {
					log.Printf("Error guardando PDF: %v", err)
				} else {
					fmt.Printf("\n✓  PDF firmado guardado en: %s (%d bytes)\n", outPath, len(blob[idx:]))
				}
			} else {
				// Guardar el blob completo si no se encuentra el header PDF.
				os.WriteFile(outPath+".blob", blob, 0644)
				fmt.Printf("\n✓  Blob guardado en: %s.blob (%d bytes)\n", outPath, len(blob))
			}
			fmt.Fprint(w, "OK")
		} else if op == "get" {
			fmt.Fprint(w, "")
		} else {
			http.Error(w, "op desconocida", 400)
		}
	})

	base := fmt.Sprintf("http://localhost:%s", port)
	rtURL := url.QueryEscape(base + "/rt")
	stURL := url.QueryEscape(base + "/st")

	gdiURI := fmt.Sprintf(
		"gdifirma://sign?ver=1_0&fileid=%s&rtservlet=%s&stservlet=%s&id=%s&keystore=PKCS11",
		fileID, rtURL, stURL, sessionID,
	)

	fmt.Println("\n─────────────────────────────────────────────────────")
	fmt.Printf("Servidor escuchando en %s\n\n", base)
	fmt.Println("Abrí esta URL en Chrome para disparar el firmador:")
	fmt.Printf("\n  %s\n\n", gdiURI)
	fmt.Println("O abrí test.html en Chrome (se genera en el directorio actual).")
	fmt.Println("─────────────────────────────────────────────────────")

	// Generar una página HTML para abrir fácilmente.
	writeTestHTML(gdiURI)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func writeTestHTML(uri string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <title>Test FirmadorGDI</title>
  <style>
    body { font-family: 'Segoe UI', sans-serif; background: #0F172A; color: #F1F5F9;
           display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { background: #1E293B; border-radius: 12px; padding: 40px; text-align: center; max-width: 420px; }
    h1 { font-size: 22px; margin-bottom: 8px; }
    p  { color: #94A3B8; font-size: 14px; margin-bottom: 28px; }
    a  { display: inline-block; background: #0EA5E9; color: white; text-decoration: none;
         padding: 12px 32px; border-radius: 8px; font-weight: 600; font-size: 15px; }
    a:hover { background: #0284C7; }
    .note { color: #64748B; font-size: 12px; margin-top: 20px; }
  </style>
</head>
<body>
  <div class="card">
    <h1>🔐 Test FirmadorGDI</h1>
    <p>Hacé clic para disparar el firmador con un documento de prueba.</p>
    <a href="%s">Firmar documento de prueba</a>
    <p class="note">El PDF firmado se guarda como <code>test_signed.pdf</code> en la carpeta del servidor.</p>
  </div>
</body>
</html>`, uri)

	path, _ := filepath.Abs("test.html")
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		log.Printf("No se pudo generar test.html: %v", err)
		return
	}
	_ = io.Discard // silenciar lint
	fmt.Printf("test.html generado en: %s\n", path)
}
