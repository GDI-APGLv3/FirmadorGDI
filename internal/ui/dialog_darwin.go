//go:build darwin

package ui

// TODO Sprint 1 Mac — implementar diálogo Cocoa nativo.
//
// Ruta recomendada (2 fases):
//
// FASE A — PoC Mac (1 día): osascript
//   osascript es AppleScript built-in en todo macOS desde 10.0.
//   Muestra un diálogo nativo del SO con campo de contraseña oculta.
//   Cero deps. Perfecto para validar el flujo Mac antes de invertir en Cocoa.
//
//   Implementación:
//     cmd := exec.Command("osascript", "-e",
//       `display dialog "PIN del token\n`+info.Label+`" default answer "" with hidden answer`)
//     out, _ := cmd.Output()
//     // parsear "button returned:Firmar, text returned:••••"
//
//   Limitación: no se puede personalizar el icono ni los colores. Aceptable para PoC.
//
// FASE B — Producción Mac: Cocoa via CGO
//   NSAlert con accessoryView = NSSecureTextField
//   Requiere Objective-C mezclado con CGO.
//   Build tag: //go:build darwin,cgo
//
//   Esqueleto CGO:
//   /*
//   #import <Cocoa/Cocoa.h>
//   char* showPINDialog(const char* label, const char* serial) {
//       NSAlert *alert = [[NSAlert alloc] init];
//       [alert setMessageText:@"Firma Digital — FirmadorGDI"];
//       NSSecureTextField *input = [[NSSecureTextField alloc] initWithFrame:NSMakeRect(0,0,200,24)];
//       [alert setAccessoryView:input];
//       [alert addButtonWithTitle:@"Firmar"];
//       [alert addButtonWithTitle:@"Cancelar"];
//       NSModalResponse r = [alert runModal];
//       if (r == NSAlertFirstButtonReturn) {
//           return strdup([[input stringValue] UTF8String]);
//       }
//       return NULL;
//   }
//   */
//   import "C"
//
// Refs:
//   https://developer.apple.com/documentation/appkit/nsalert
//   https://pkg.go.dev/cmd/cgo

import (
	"fmt"
	"os/exec"
	"strings"
)

func ShowInfoDialog(title, message string) {
	script := fmt.Sprintf(`display dialog "%s" buttons {"Aceptar"} default button "Aceptar"`, message)
	_ = exec.Command("osascript", "-e", script).Run()
}

func ShowErrorDialog(title, message string) {
	script := fmt.Sprintf(`display dialog "%s" buttons {"Aceptar"} default button "Aceptar" with icon stop`, message)
	_ = exec.Command("osascript", "-e", script).Run()
}

func ShowPINDialog(info TokenInfo) (PINResult, error) {
	script := fmt.Sprintf(
		`display dialog "Token: %s\nCUIL: %s\nVálido: %s\n\nIngresá el PIN:" `+
			`default answer "" with hidden answer `+
			`buttons {"Cancelar", "Firmar"} default button "Firmar"`,
		info.Label, info.SerialNumber, info.ValidUntil,
	)

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		// El usuario canceló (osascript devuelve exit 1 al cancelar).
		return PINResult{Cancelled: true}, ErrCancelled
	}

	// Output: "button returned:Firmar, text returned:ELPIN"
	result := string(out)
	for _, part := range strings.Split(result, ", ") {
		if strings.HasPrefix(part, "text returned:") {
			pin := strings.TrimPrefix(part, "text returned:")
			pin = strings.TrimSpace(pin)
			if pin == "" {
				return PINResult{Cancelled: true}, ErrCancelled
			}
			return PINResult{PIN: pin}, nil
		}
	}
	return PINResult{Cancelled: true}, ErrCancelled
}
