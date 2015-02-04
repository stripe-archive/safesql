SafeSQL
=======

SafeSQL is a static analysis tool for Go that protects against SQL injections.


How does it work?
-----------------

SafeSQL uses the static analysis utilities in [go/tools][tools] to search for
all call sites of each of the `query` functions in package [database/sql][sql]
(i.e., functions which accept a `string` parameter named `query`). It then makes
sure that every such call site uses a query that is a compile-time constant.

The principle behind SafeSQL's safety guarantees is that queries that are
compile-time constants cannot be subverted by user-supplied data: they must
either incorporate no user-controlled values, or incorporate them using the
package's safe placeholder mechanism. In particular, call sites which build up
SQL statements via `fmt.Sprintf` or string concatenation or other mechanisms
will not be allowed.

[tools]: https://godoc.org/golang.org/x/tools/go
[sql]: http://golang.org/pkg/database/sql/

False positives
---------------

If SafeSQL passes, your application is free from SQL injections (modulo bugs in
the tool), however there are a great many safe programs which SafeSQL will
declare potentially unsafe. These false positives fall roughly into two buckets:

First, SafeSQL does not currently recursively trace functions through the call
graph. If you have a function that looks like this:

    func MyQuery(query string, args ...interface{}) (*sql.Rows, error) {
            return globalDBObject.Query(query, args...)
    }

and only call `MyQuery` with compile-time constants, your program is safe;
however SafeSQL will report that `(*database/sql.DB).Query` is called with a
non-constant parameter (namely the parameter to `MyQuery`). This is by no means
a fundamental limitation: SafeSQL could recursively trace the `query` argument
through every intervening helper function to ensure that its argument is always
constant, but this code has yet to be written.

If you use a wrapper for `database/sql` (e.g., [`sqlx`][sqlx]), it's likely
SafeSQL will not work for you because of this.

The second sort of false positive is based on a limitation in the sort of
analysis SafeSQL performs: there are many safe SQL statements which are not
feasible (or not possible) to represent as compile-time constants. More advanced
static analysis techniques (such as taint analysis) or user-provided safety
annotations would be able to reduce the number of false positives, but this is
expected to be a significant undertaking.

[sqlx]: https://github.com/jmoiron/sqlx
