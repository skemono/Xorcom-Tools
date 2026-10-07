# Utilidades XORCOM

Herramientas de escritorio (Windows) para automatizar la configuración de Xorcom CompletePBX 5.
Cada herramienta es un formulario del talonario: F-01 Conexiones, F-02 …

## Desarrollo

- Requisitos: Go 1.27, Node 24, `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.28`
- `wails3 dev` · `go test ./pbx/...` · `wails3 build` → `bin/UtilidadesXorcom.exe`

## Agregar una herramienta

1. `foo.go`: un servicio Go; regístrelo en `main.go` (`application.NewService`).
2. `frontend/src/modules/Foo.svelte`: el formulario; agréguelo a `frontend/src/modules.ts` con su código `F-0N`.

## Publicar una versión

`git tag v0.1.0 && git push origin v0.1.0`: el workflow compila, publica `UtilidadesXorcom_windows_amd64.zip` y `checksums.txt`, y la app se actualiza sola al abrirse.

## Credenciales

Nunca en el repositorio (es público). Las contraseñas de cada PBX van al Administrador de credenciales de Windows desde la app; las notas locales en `docs/reference/` están en `.gitignore`.

Diseño: [spec](docs/superpowers/specs/2026-10-07-utilidades-xorcom-design.md) · [DESIGN.md](DESIGN.md) · [PRODUCT.md](PRODUCT.md)
