package main

import (
	"database/sql"
	"fmt"
)

func main() {
	fmt.Println(query("'test' OR 1=1"))
	fmt.Println(query2("'test' OR 1=1"))
}

const GetAllQuery = "SELECT COUNT(*) FROM t WHERE arg=%s"

// For this test we expect the second QueryRow to be an issue even though the line before has a comment
func query2(arg string) error {
	db, err := sql.Open("postgres", "postgresql://test:test@test")
	if err != nil {
		return err
	}

	query := fmt.Sprintf(GetAllQuery, arg)
	_ := db.QueryRow(query) //nolint:safesql
	_ := db.QueryRow(fmt.Sprintf(GetAllQuery, "Catch me please?")) //nolint:safesql


	return nil
}