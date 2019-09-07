package main

import (
	"context"
	"database/sql"
	"log"
)

var (
	ctx context.Context
	db *sql.DB
)

func runDbQuery(db *sql.DB) {
	sqlStmt := `
	create table foo (id integer not null primary key, name text);
	delete from foo;
	`

	if _, err := db.Exec(sqlStmt); err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
}

func main() {
	runDbQuery(db)

	log.Printf("holy moley\n")
}
