package stats

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"testing"
)

type suggestionConnector struct {
	t     *testing.T
	calls int
}

func (c *suggestionConnector) Connect(context.Context) (driver.Conn, error) {
	return &suggestionConn{c}, nil
}
func (c *suggestionConnector) Driver() driver.Driver { return nil }

type suggestionConn struct{ c *suggestionConnector }

func (c *suggestionConn) Prepare(string) (driver.Stmt, error) { panic("unexpected prepare") }
func (c *suggestionConn) Close() error                        { return nil }
func (c *suggestionConn) Begin() (driver.Tx, error)           { panic("unexpected transaction") }
func (c *suggestionConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.c.calls++
	if _, ok := ctx.Deadline(); !ok {
		c.c.t.Fatal("query has no timeout")
	}
	if args[0].Value != "server2" {
		c.c.t.Fatalf("wrong server: %v", args)
	}
	if !strings.Contains(query, "player_server_profiles") || !strings.Contains(query, "escape '!'") || !strings.Contains(query, "limit ?") {
		c.c.t.Fatalf("query lacks directory constraints: %s", query)
	}
	if c.c.calls == 1 {
		if args[1].Value != "Lc!_%" || args[2].Value != int64(10) {
			c.c.t.Fatalf("prefix arguments: %v", args)
		}
		return &suggestionRows{names: []string{"Lc_One", "Lc_Two"}}, nil
	}
	if !strings.Contains(query, "not like") || args[1].Value != "%Lc!_%" || args[2].Value != "Lc!_%" || args[3].Value != int64(8) {
		c.c.t.Fatalf("substring arguments: %v, %s", args, query)
	}
	return &suggestionRows{names: []string{"OtherLc_Three"}}, nil
}

type suggestionRows struct {
	names []string
	index int
}

func (r *suggestionRows) Columns() []string { return []string{"player_name"} }
func (r *suggestionRows) Close() error      { return nil }
func (r *suggestionRows) Next(dest []driver.Value) error {
	if r.index == len(r.names) {
		return io.EOF
	}
	dest[0] = r.names[r.index]
	r.index++
	return nil
}

func TestPlayerSuggestionsScopeOrderingAndLiteralWildcard(t *testing.T) {
	connector := &suggestionConnector{t: t}
	db := sql.OpenDB(connector)
	defer db.Close()
	service := &Service{db: db}
	names, err := service.PlayerSuggestions(context.Background(), " server2 ", " Lc_ ")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "Lc_One,Lc_Two,OtherLc_Three" || connector.calls != 2 {
		t.Fatalf("unexpected names/calls: %v, %d", names, connector.calls)
	}
}
func TestPlayerSuggestionsValidateBeforeDatabaseAccess(t *testing.T) {
	service := &Service{}
	for _, server := range []string{"", "all", "total", strings.Repeat("x", 65)} {
		if _, err := service.PlayerSuggestions(context.Background(), server, "Alex"); err == nil {
			t.Fatalf("accepted invalid server %q", server)
		}
	}
	if _, err := service.PlayerSuggestions(context.Background(), "main", strings.Repeat("中", 65)); err == nil {
		t.Fatal("accepted excessive input")
	}
	names, err := service.PlayerSuggestions(context.Background(), "main", " ")
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("empty input: %v %v", names, err)
	}
	if got := escapePlayerSuggestionLike("A!_%中"); got != "A!!!_!%中" {
		t.Fatalf("literal escaping: %q", got)
	}
}
