// nolint (WIP)
package queries_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"go-simpler.org/queries"
	// <database driver of your choice>
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	db, err := sql.Open("<driver name>", "<connection string>")
	if err != nil {
		return err
	}

	columns := []string{"first_name", "last_name"}
	if true {
		columns = append(columns, "created_at")
	}

	var qb queries.Builder
	qb.Appendf("select %s from users", strings.Join(columns, ", "))
	if true {
		qb.Appendf(" where created_at >= %$", time.Date(2024, time.January, 1, 0, 0, 0, 0, time.Local))
	}

	// select first_name, last_name, created_at from users where created_at >= $1
	rows, err := db.QueryContext(ctx, qb.String(), qb.Args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var users []struct {
		FirstName string    `sql:"first_name"`
		LastName  string    `sql:"last_name"`
		CreatedAt time.Time `sql:"created_at"`
	}
	if err := queries.ScanAll(&users, rows); err != nil {
		return err
	}

	fmt.Println(users)
	return nil
}
