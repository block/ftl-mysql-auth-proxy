package mysqlauthproxy

import (
	"context"
	"database/sql"
	"log"
	"net/url"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
)

func TestMysqlAuthProxy(t *testing.T) {
	parse, err := url.Parse("http://localhost:3232")
	assert.NoError(t, err)
	proxy := NewProxy(*parse, "admin:admin@tcp(127.0.0.1:3306)/mydatabase", defaultLogger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := proxy.ListenAndServe(ctx)
		if err != nil {
			t.Errorf("error: %v", err)
		}
	}()
	time.Sleep(time.Second)

	// Open a connection to the database
	db, err := sql.Open("mysql", "foo:bar@tcp(127.0.0.1:3232)/mydatabase")
	if err != nil {
		log.Fatal("Error opening database connection:", err)
	}
	defer db.Close()

	// Verify the connection
	res, err := db.Query("SELECT 1")
	assert.NoError(t, err)
	assert.True(t, res.Next())
	result := 0
	err = res.Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

}
