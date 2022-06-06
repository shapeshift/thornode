package openapi

import (
	_ "embed"
	"net/http"

	"github.com/rs/zerolog/log"
)

// -------------------------------------------------------------------------------------
// Config
// -------------------------------------------------------------------------------------

var (
	//go:embed openapi.yaml
	openapiYAML []byte

	swaggerUI = []byte(`
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta
      name="description"
      content="SwaggerUI"
    />
    <title>SwaggerUI</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
  </head>
  <body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: window.location.pathname + '/openapi.yaml',
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "StandaloneLayout",
      });
    };
  </script>
  </body>
</html>
	`)
)

// -------------------------------------------------------------------------------------
// Handlers
// -------------------------------------------------------------------------------------

func HandleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/yaml")
	_, err := w.Write(openapiYAML)
	if err != nil {
		log.Warn().Err(err).Msg("failed to write spec response")
	}
}

func HandleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/html")
	_, err := w.Write(swaggerUI)
	if err != nil {
		log.Warn().Err(err).Msg("failed to write swagger ui response")
	}
}
