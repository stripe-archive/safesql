package main

import (
	"database/sql"
	"fmt"
)

// For this test we expect the second QueryRow to be an issue even though the line before has a comment
func query(arg string) error {
	db, err := sql.Open("postgres", "postgresql://test:test@test")
	if err != nil {
		return err
	}

	query := fmt.Sprintf(GetAllQuery, arg)
	_ := db.QueryRow(query) //nolint:safesql
	_ := db.QueryRow(fmt.Sprintf(GetAllQuery, "Catch me please?")) //nolint:safesql


	return nil
}