package rest

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gorilla/mux"
	"github.com/rakyll/statik/fs"

	_ "gitlab.com/thorchain/thornode/docs/statik"
	"gitlab.com/thorchain/thornode/x/thorchain/query"
)

const (
	restURLParam  = "param1"
	restURLParam2 = "param2"
)

// RegisterRoutes - Central function to define routes that get registered by the main application
func RegisterRoutes(cliCtx client.Context, r *mux.Router, storeName string) {
	// Health Check Endpoint
	r.HandleFunc(
		fmt.Sprintf("/%s/ping", storeName),
		pingHandler(cliCtx, storeName),
	).Methods(http.MethodGet, http.MethodOptions)

	// limit api calls
	// limit it to 60 per minute
	lmt := tollbooth.NewLimiter(60, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	lmt.SetMessage("You have reached maximum request limit.")

	// Dynamically create endpoints of all funcs in querier.go
	for _, q := range query.Queries {
		endpoint := q.Endpoint(storeName, restURLParam, restURLParam2)
		if endpoint != "" { // don't setup REST endpoint if THORNode have no endpoint
			r.Handle(
				endpoint,
				tollbooth.LimitFuncHandler(
					lmt,
					getHandlerWrapper(q, storeName, cliCtx),
				),
			).Methods(http.MethodGet, http.MethodOptions)
		}
	}

	// Get unsigned json for emitting a transaction. Validators only.
	r.HandleFunc(
		fmt.Sprintf("/%s/txs", storeName),
		postTxsHandler(cliCtx),
	).Methods(http.MethodPost)

	r.HandleFunc("/txs",
		BroadcastTxRequest(cliCtx),
	).Methods(http.MethodPost)

	r.HandleFunc(
		fmt.Sprintf("/%s/tss", storeName),
		newTssPoolHandler(cliCtx),
	).Methods(http.MethodPost)

	r.HandleFunc(
		fmt.Sprintf("/%s/errata", storeName),
		newErrataTxHandler(cliCtx),
	).Methods(http.MethodPost)

	r.HandleFunc(
		fmt.Sprintf("/%s/deposit", storeName),
		newDepositHandler(cliCtx),
	).Methods(http.MethodPost)

	r.Use(mux.CORSMethodMiddleware(r))
	r.Use(customCORSHeader())
	registerSwaggerAPI(storeName, r)
}

// RegisterSwaggerAPI registers swagger route with API Server
func registerSwaggerAPI(storeName string, rtr *mux.Router) {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}
	pathPrefix := fmt.Sprintf("/%s/doc/", storeName)
	staticServer := http.FileServer(statikFS)
	rtr.PathPrefix(pathPrefix).Handler(http.StripPrefix(pathPrefix, staticServer))
}

func customCORSHeader() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			next.ServeHTTP(w, req)
		})
	}
}
