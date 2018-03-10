package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/dgraph-io/dgraph/client"
	"github.com/dgraph-io/dgraph/protos/api"
	"github.com/go-chi/chi"
)

const (
	TypeJSON = "application/json"
)

func makeAPIHandler() http.Handler {
	mux := chi.NewRouter()
	mux.Use(transactionMiddleware)

	mux.Get("/api/query", queryHandler)

	// mutation api
	mux.Get("/api/nodes/{type}/{id}", readHandler)
	mux.Post("/api/nodes/{type}", mutateHandler)
	mux.Put("/api/nodes/{type}/{id}", mutateHandler)
	mux.Delete("/api/nodes/{type}/{id}", deleteHandler)

	// TODO allow to delete triples from graph
	// TODO consider to expose raw api for admin users

	// edges api

	return mux
}

func transactionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := newDgraphClient()
		if err != nil {
			sendError(w, err)
			return
		}

		tx := c.NewTxn()
		defer tx.Discard(r.Context())

		ctx := context.WithValue(r.Context(), "tx", tx)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func transaction(r *http.Request) *client.Txn {
	return r.Context().Value("tx").(*client.Txn)
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	query, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, err)
		return
	}

	tx := transaction(r)

	resp, err := tx.Query(r.Context(), string(query))
	if err != nil {
		sendError(w, err)
		return
	}

	w.Header().Set("Content-Type", TypeJSON)
	w.Write(resp.GetJson())
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	// TODO read all values of given node
}

func mutateHandler(w http.ResponseWriter, r *http.Request) {
	tx := transaction(r)

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, err)
		return
	}

	resp, err := tx.Mutate(r.Context(), &api.Mutation{
		SetJson:   data,
		CommitNow: true,
	})
	if err != nil {
		sendError(w, err)
		return
	}

	sendJSON(w, resp)

	// TODO notify about change
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	tx := transaction(r)
	id := chi.URLParam(r, "id")

	resp, err := tx.Mutate(r.Context(), &api.Mutation{
		DelNquads: []byte("<" + id + "> * * .\n"),
		CommitNow: true,
	})
	if err != nil {
		sendError(w, err)
		return
	}

	sendJSON(w, resp)

	// TODO notify about change
}

func sendJSON(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", TypeJSON)

	marshaller, ok := result.(json.Marshaler)
	if ok {
		b, err := marshaller.MarshalJSON()
		if err != nil {
			// TODO check whether it is possible to send error at this phase
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.Write(b)
		return
	}

	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		// TODO check whether it is possible to send error at this phase
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func sendError(w http.ResponseWriter, err error) {
	sendJSON(w, &struct {
		Error string `json:"error"`
	}{
		Error: err.Error(),
	})
}
