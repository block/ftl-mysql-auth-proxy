package mysqlauthproxy

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"testing"

	"github.com/alecthomas/assert/v2"
	_ "github.com/go-sql-driver/mysql"
)

func TestMysqlAuthProxy(t *testing.T) {
	portC := make(chan int)
	dsnFunc := func(ctx context.Context) (string, error) {
		return "admin:admin@tcp(127.0.0.1:3306)/mydatabase", nil
	}
	proxy := NewProxy("localhost", 0, dsnFunc, defaultLogger, portC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := proxy.ListenAndServe(ctx)
		if err != nil {
			t.Errorf("error: %v", err)
		}
	}()
	port := <-portC

	// Open a connection to the database
	db, err := sql.Open("mysql", fmt.Sprintf("foo:bar@tcp(127.0.0.1:%d)/mydatabase", port))
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
