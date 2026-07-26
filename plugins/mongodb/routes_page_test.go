package mongodb

import (
	"encoding/base64"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestDocumentCursorRoundTrip(t *testing.T) {
	ids := map[string]any{"objectID": bson.NewObjectID(), "string": "ada", "int": int64(7)}
	for name, id := range ids {
		t.Run(name, func(t *testing.T) {
			cursor, err := encodeDocumentCursor(id)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := decodeDocumentCursor(cursor)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if decoded != id {
				t.Fatalf("round trip: got %#v, want %#v", decoded, id)
			}
		})
	}
}

func TestDecodeDocumentCursorRejectsGarbage(t *testing.T) {
	empty := "id:" + base64.RawURLEncoding.EncodeToString([]byte("{}"))
	for _, raw := range []string{"12", "id:!!", empty} {
		if _, err := decodeDocumentCursor(raw); !errors.Is(err, plugin.ErrInvalidInput) {
			t.Fatalf("cursor %q: want invalid input, got %v", raw, err)
		}
	}
}

func TestAfterIDFilter(t *testing.T) {
	unfiltered := afterIDFilter(bson.M{}, "ada")
	if page, ok := unfiltered["_id"].(bson.M); !ok || page["$gt"] != "ada" {
		t.Fatalf("empty filter: got %#v", unfiltered)
	}
	combined := afterIDFilter(bson.M{"name": "Ada"}, "ada")
	clauses, ok := combined["$and"].([]any)
	if !ok || len(clauses) != 2 {
		t.Fatalf("filtered: got %#v", combined)
	}
	if user, ok := clauses[0].(bson.M); !ok || user["name"] != "Ada" {
		t.Fatalf("user filter dropped: %#v", clauses[0])
	}
}
