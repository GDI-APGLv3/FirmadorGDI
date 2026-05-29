"""
make-ico-bmp.py
Genera firmadorgdi.ico y convierte dialog.png + banner.png a BMP.
Requiere: Pillow
"""

from PIL import Image, ImageDraw, ImageFont
import os
import sys

DIR = r"C:\Users\santi\OneDrive\GDILatam\APP-GDILatam\FirmadorGDI\installer"
LOGO_PATH = r"C:\Users\santi\OneDrive\GDILatam\Operations\Comunicacion\01-Manual-Marca\GDI\logos\gdi-logo-3.png"

# ── Colores GDI ──────────────────────────────────────────────────────────────
GDI_DARK   = (13, 19, 56)      # #0d1338
GDI_INDIGO = (22, 21, 140)     # #16158C
SKY        = (14, 165, 233)    # #0ea5e9
WHITE      = (255, 255, 255)

# ─────────────────────────────────────────────────────────────────────────────
# 1. PNG → BMP (dialog 493x312 y banner 493x58)
# ─────────────────────────────────────────────────────────────────────────────

def png_to_bmp(src, dst, expected_size):
    img = Image.open(src).convert("RGB")
    w, h = img.size
    if (w, h) != expected_size:
        print(f"  WARN: {src} es {w}x{h}, se esperaba {expected_size[0]}x{expected_size[1]}. Redimensionando...")
        img = img.resize(expected_size, Image.LANCZOS)
    img.save(dst, format="BMP")
    print(f"  BMP: {dst}  ({expected_size[0]}x{expected_size[1]})")

dialog_png = os.path.join(DIR, "dialog.png")
dialog_bmp = os.path.join(DIR, "dialog.bmp")
banner_png = os.path.join(DIR, "banner.png")
banner_bmp = os.path.join(DIR, "banner.bmp")

if not os.path.exists(dialog_png):
    sys.exit(f"ERROR: No se encontró {dialog_png}. Ejecutar render-assets.mjs primero.")
if not os.path.exists(banner_png):
    sys.exit(f"ERROR: No se encontró {banner_png}. Ejecutar render-assets.mjs primero.")

png_to_bmp(dialog_png, dialog_bmp, (493, 312))
png_to_bmp(banner_png, banner_bmp, (493, 58))

# ─────────────────────────────────────────────────────────────────────────────
# 2. Generar firmadorgdi.ico
#    Tamaños: 16, 32, 48, 256 px
#    Diseño: fondo indigo GDI, letra "G" en blanco, punto sky abajo-derecha
# ─────────────────────────────────────────────────────────────────────────────

def make_icon_base(size):
    """
    Dibuja el ícono base a alta resolución y lo escala.
    Usa el logo PNG si está disponible; si no, construye uno geométrico.
    """
    # Trabajamos en 256x256 y luego escalamos
    canvas = 256
    img = Image.new("RGBA", (canvas, canvas), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Fondo: círculo con gradiente simulado (dos capas)
    # Capa base oscura
    draw.ellipse([0, 0, canvas-1, canvas-1], fill=(*GDI_DARK, 255))
    # Capa indigo centrada
    margin = 8
    draw.ellipse([margin, margin, canvas-1-margin, canvas-1-margin], fill=(*GDI_INDIGO, 255))

    # Intentar pegar el logo GDI si tiene transparencia (RGBA)
    logo_ok = False
    if os.path.exists(LOGO_PATH):
        try:
            logo = Image.open(LOGO_PATH).convert("RGBA")
            # El logo es blanco sobre transparente (o coloreado)
            # Lo redimensionamos al 60% del canvas, centrado
            logo_size = int(canvas * 0.60)
            logo = logo.resize((logo_size, logo_size), Image.LANCZOS)
            # Convertir a blanco puro manteniendo alpha
            r, g, b, a = logo.split()
            logo_white = Image.merge("RGBA", (
                Image.new("L", logo.size, 255),
                Image.new("L", logo.size, 255),
                Image.new("L", logo.size, 255),
                a
            ))
            offset = (canvas - logo_size) // 2
            img.paste(logo_white, (offset, offset), logo_white)
            logo_ok = True
        except Exception as e:
            print(f"  WARN: No se pudo usar el logo PNG: {e}")

    if not logo_ok:
        # Fallback: letra "G" estilizada
        try:
            font = ImageFont.truetype("C:/Windows/Fonts/arialbd.ttf", int(canvas * 0.55))
        except:
            font = ImageFont.load_default()
        text = "G"
        bbox = draw.textbbox((0, 0), text, font=font)
        tw, th = bbox[2] - bbox[0], bbox[3] - bbox[1]
        tx = (canvas - tw) // 2 - bbox[0]
        ty = (canvas - th) // 2 - bbox[1] - int(canvas * 0.03)
        draw.text((tx, ty), text, fill=(*WHITE, 255), font=font)

    # Punto sky (acento de marca) — esquina inferior derecha
    dot_r = int(canvas * 0.14)
    dot_center = int(canvas * 0.76)
    draw.ellipse(
        [dot_center - dot_r, dot_center - dot_r,
         dot_center + dot_r, dot_center + dot_r],
        fill=(*SKY, 255)
    )
    # Punto oscuro pequeño dentro del punto sky (efecto anillo)
    inner_r = int(dot_r * 0.45)
    draw.ellipse(
        [dot_center - inner_r, dot_center - inner_r,
         dot_center + inner_r, dot_center + inner_r],
        fill=(*GDI_DARK, 200)
    )

    # Escalar al tamaño final
    if size != canvas:
        img = img.resize((size, size), Image.LANCZOS)

    return img


# Generar tamaños
sizes = [16, 32, 48, 256]
frames = [make_icon_base(s) for s in sizes]

ico_path = os.path.join(DIR, "firmadorgdi.ico")
frames[0].save(
    ico_path,
    format="ICO",
    sizes=[(s, s) for s in sizes],
    append_images=frames[1:],
)
print(f"  ICO: {ico_path}  ({', '.join(str(s) for s in sizes)} px)")

print("\nTodos los assets generados correctamente.")
