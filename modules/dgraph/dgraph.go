package dgraph

import (
	"context"
	"google.golang.org/grpc/metadata"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/dgraph-io/dgo"
	"github.com/dgraph-io/dgo/protos/api"
	"github.com/sergeyt/pandora/modules/apiutil"
	"github.com/sergeyt/pandora/modules/config"
	"google.golang.org/grpc"
)

func NewClient() (*dgo.Dgraph, error) {
	d, err := grpc.Dial(config.DB.Addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return dgo.NewDgraphClient(
		api.NewDgraphClient(d),
	), nil
}

// TODO incremental update of schema
func InitSchema() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// TODO configurable path to schema
	schema, err := ioutil.ReadFile("./schema.txt")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	token := os.Getenv("DGRAPH_TOKEN")
	if len(token) > 0 {
		md := metadata.New(nil)
		md.Append("auth-token", token)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	err = c.Alter(ctx, &api.Operation{
		Schema: string(schema),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func TransactionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := NewClient()
		if err != nil {
			apiutil.SendError(w, err)
			return
		}

		tx := c.NewTxn()
		defer tx.Discard(r.Context())

		ctx := context.WithValue(r.Context(), "tx", tx)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func RequestTransaction(r *http.Request) *dgo.Txn {
	return r.Context().Value("tx").(*dgo.Txn)
}
