package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"nhooyr.io/websocket"
	"os"
	"path/filepath"
	"player-stats-backend-go/internal/auth"
	"player-stats-backend-go/internal/config"
	"player-stats-backend-go/internal/httpapi"
	"player-stats-backend-go/internal/importer"
	"player-stats-backend-go/internal/settings"
	"player-stats-backend-go/internal/share"
	"player-stats-backend-go/internal/stats"
	"player-stats-backend-go/internal/storage"
	"strings"
	"testing"
	"time"
)

type publicFixture struct {
	db         *storage.DB
	handler    http.Handler
	shares     *share.Service
	key, admin string
}

func newPublicFixture(t *testing.T) publicFixture {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	cfg := config.Load()
	cfg.DatabasePath = filepath.Join(root, "database/player.db")
	cfg.MainDirectory = filepath.Join(root, "main")
	cfg.SubDirectory = filepath.Join(root, "sub")
	cfg.LogArchiveDirectory = root
	cfg.LiveLogSnapshotDirectory = filepath.Join(root, "live")
	cfg.XrayTempDirectory = filepath.Join(root, "xray")
	cfg.AdminPassword = "integration-only-password"
	db, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = storage.EnsureDefaults(ctx, db, cfg); e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{cfg.MainDirectory, cfg.SubDirectory} {
		if e = os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
	}
	body := "日期,时间,玩家,行为,x,y,z,维度,x2,y2,z2,维度2,详情1,详情2\n"
	for i := 0; i < 260; i++ {
		body += fmt.Sprintf("2026-04-01,19:19:%02d,Älex,破坏方块,0,10,0,overworld,0,10,0,overworld,minecraft:diamond_ore,\n", i%60)
	}
	body += "2026-04-01,19:20:00,Älex,破坏方块,0,10,0,overworld,0,10,0,overworld,minecraft:oak_log,\n2026-04-01,19:21:00,Älex,放置方块,0,10,0,overworld,0,10,0,overworld,minecraft:oak_sapling,\n"
	if e = os.WriteFile(filepath.Join(cfg.MainDirectory, "player_actions_2026-04-01.csv"), []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(cfg.SubDirectory, "player_actions_2026-04-01.csv"), []byte("日期,时间,玩家,行为,x,y,z,维度,x2,y2,z2,维度2,详情1,详情2\n2026-04-01,19:19:00,äLEX,放置方块,0,10,0,overworld,0,10,0,overworld,minecraft:stone,\n"), 0600); e != nil {
		t.Fatal(e)
	}
	policy := settings.NewService(db, cfg)
	ingest := importer.NewService(db, cfg, policy)
	t.Cleanup(ingest.CloseLiveLogSnapshots)
	if _, e = ingest.ImportFromConfiguredSource(ctx, "", true, nil); e != nil {
		t.Fatal(e)
	}
	statsSvc := stats.NewService(db, cfg)
	logs := importer.NewLogQueryService(ingest)
	shares := share.NewService(db, cfg, policy, statsSvc, nil, logs)
	accounts := auth.NewService(db, cfg)
	if e = accounts.EnsureAdmin(ctx); e != nil {
		t.Fatal(e)
	}
	login, e := accounts.Login(ctx, auth.LoginRequest{Username: cfg.AdminUsername, Password: cfg.AdminPassword})
	if e != nil {
		t.Fatal(e)
	}
	var key string
	if e = db.QueryRowContext(ctx, `select astrbot_api_key from sharing_settings`).Scan(&key); e != nil {
		t.Fatal(e)
	}
	app := httpapi.NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), accounts, policy, shares, ingest, nil, logs, nil, statsSvc, nil, nil)
	return publicFixture{db: db, handler: app.Handler(), shares: shares, key: key, admin: login.Token}
}
func (f publicFixture) request(t *testing.T, method, path string, body any, identity string, want int, out any) {
	t.Helper()
	var payload []byte
	var e error
	if body != nil {
		payload, e = json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if identity == "plugin" {
		req.Header.Set(settings.AstrBotAPIKeyHeader, f.key)
	} else if identity == "admin" {
		req.Header.Set("Authorization", "Bearer "+f.admin)
	} else if identity == "wrong" {
		req.Header.Set(settings.AstrBotAPIKeyHeader, "wrong-key")
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, want, rec.Code, rec.Body.String())
	}
	if out != nil {
		if e = json.Unmarshal(rec.Body.Bytes(), out); e != nil {
			t.Fatal(e)
		}
	}
	if identity == "" && want == 200 {
		for _, secret := range []string{f.key, f.admin, "password_hash", "smb_password"} {
			if strings.Contains(rec.Body.String(), secret) {
				t.Fatal("public response leaked a secret")
			}
		}
	}
}
func TestSQLitePublicReportsAndPluginHTTP(t *testing.T) {
	f := newPublicFixture(t)
	ctx := context.Background()
	t.Run("plugin auth and access boundary", func(t *testing.T) {
		for _, identity := range []string{"", "wrong"} {
			f.request(t, "GET", "/api/stats/player?playerName=alex", nil, identity, 401, nil)
			f.request(t, "POST", "/api/share/tokens", map[string]string{"playerName": "Älex"}, identity, 401, nil)
		}
		for _, path := range []string{"/api/admin/settings/smb", "/api/stats/overview", "/api/import/files", "/api/stats/player-suggestions?q=a"} {
			f.request(t, "GET", path, nil, "plugin", 401, nil)
		}
		f.request(t, "GET", "/api/stats/player?playerName=%C3%A4LEX", nil, "plugin", 200, nil)
		f.request(t, "GET", "/api/stats/player-presence?playerName=%C3%A4LEX", nil, "plugin", 200, nil)
		f.request(t, "GET", "/api/stats/players?serverId=main", nil, "plugin", 200, nil)
		f.request(t, "GET", "/api/stats/public-coordinate-logs?serverId=main&x=0&y=10&z=0", nil, "plugin", 200, nil)
	})
	tokens := map[string]string{}
	t.Run("player report resources milestones and two sources", func(t *testing.T) {
		var created share.ShareTokenResponse
		f.request(t, "POST", "/api/share/tokens", map[string]string{"playerName": "äLEX"}, "plugin", 201, &created)
		tokens["player"] = created.Token
		var got share.PlayerDetailsResponse
		f.request(t, "GET", "/api/share/"+created.Token, nil, "", 200, &got)
		if got.PlayerName != "Älex" || len(got.Servers) != 2 {
			t.Fatalf("wrong player report %+v", got)
		}
		main := got.Servers[0]
		if main.ServerID != "main" || main.TotalCount != 262 || main.FirstSeenAt == nil || main.FirstSeenAt.Hour() != 19 {
			t.Fatalf("wrong main summary %+v", main)
		}
		var ore, wood, sapling int64
		for _, v := range main.Ores {
			ore += v.Count
		}
		for _, v := range main.Woods {
			wood += v.Count
		}
		for _, v := range main.Saplings {
			sapling += v.Count
		}
		if ore != 260 || wood != 1 || sapling != 1 || len(main.Milestones) != 7 {
			t.Fatalf("resource or milestone mismatch: %d/%d/%d", ore, wood, sapling)
		}
	})
	t.Run("total and active ranking persist date range", func(t *testing.T) {
		for _, kind := range []string{"total", "active"} {
			var created share.ShareTokenResponse
			f.request(t, "POST", "/api/share/ranking-tokens", map[string]any{"type": kind, "limit": 10}, "plugin", 201, &created)
			tokens["ranking"] = created.Token
			var got share.RankingDetailsResponse
			f.request(t, "GET", "/api/share/ranking/"+created.Token, nil, "", 200, &got)
			if got.RankingType != kind || len(got.Servers) != 2 || len(got.Servers[0].Players) != 1 || got.Servers[0].Players[0].TotalCount != 262 {
				t.Fatalf("wrong ranking %+v", got)
			}
		}
	})
	t.Run("log snapshot pagination independence and path removal", func(t *testing.T) {
		var created share.LogQueryShareTokenResponse
		f.request(t, "POST", "/api/share/log-query-tokens", map[string]any{"serverId": "main", "x": 0, "y": 10, "z": 0, "radius": 0}, "plugin", 201, &created)
		tokens["log"] = created.Token
		seen := map[int64]bool{}
		for page := 1; page <= 3; page++ {
			var got share.LogQueryShareDetailsResponse
			f.request(t, "GET", fmt.Sprintf("/api/share/log-query/%s?page=%d&pageSize=100", created.Token, page), nil, "", 200, &got)
			if got.TotalRows != 262 || got.TotalPages != 3 || got.Page != page {
				t.Fatalf("wrong page %+v", got)
			}
			for _, r := range got.Rows {
				if r.FilePath != "" || seen[r.LineNumber] {
					t.Fatal("path leak or repeated page row")
				}
				seen[r.LineNumber] = true
			}
		}
		if len(seen) != 262 {
			t.Fatal("missing rows", len(seen))
		}
		if _, e := f.db.ExecContext(ctx, `delete from import_files`); e != nil {
			t.Fatal(e)
		}
		var snapshot share.LogQueryShareDetailsResponse
		f.request(t, "GET", "/api/share/log-query/"+created.Token, nil, "", 200, &snapshot)
		if snapshot.TotalRows != 262 {
			t.Fatal("snapshot depended on parsed data")
		}
	})
	t.Run("xray snapshot and delivery state", func(t *testing.T) {
		req := share.XrayGroupSendRequest{ServerID: "main", PlayerName: "Älex", Player: &importer.XrayPlayerRiskView{PlayerName: "Älex", RiskScore: 55, RiskLevel: "MEDIUM"}}
		f.request(t, "POST", "/api/share/xray/send-to-group", req, "plugin", 401, nil)
		var created share.XrayGroupSendResponse
		f.request(t, "POST", "/api/share/xray/send-to-group", req, "admin", 201, &created)
		token := strings.TrimPrefix(created.SharePath, "/xray-share/")
		tokens["xray"] = token
		var got share.XrayShareDetailsResponse
		f.request(t, "GET", "/api/share/xray/"+token, nil, "", 200, &got)
		if got.Player.RiskScore != 55 {
			t.Fatal("wrong xray snapshot")
		}
		var pending []share.XrayGroupMessageResponse
		f.request(t, "GET", "/api/share/xray-group-messages/pending", nil, "plugin", 200, &pending)
		if len(pending) != 1 || pending[0].ID != created.MessageID {
			t.Fatal("pending message missing")
		}
		f.request(t, "POST", fmt.Sprintf("/api/share/xray-group-messages/%d/delivery", created.MessageID), map[string]bool{"success": true}, "plugin", 200, nil)
		f.request(t, "GET", "/api/share/xray-group-messages/pending", nil, "plugin", 200, &pending)
		if len(pending) != 0 {
			t.Fatal("delivered message still pending")
		}
	})
	t.Run("xray share and outbox roll back together", func(t *testing.T) {
		var before, after int
		if e := f.db.QueryRowContext(ctx, `select count(*) from share_links`).Scan(&before); e != nil {
			t.Fatal(e)
		}
		if _, e := f.db.ExecContext(ctx, `create trigger reject_test_outbox before insert on notification_outbox begin select raise(abort,'injected outbox failure'); end`); e != nil {
			t.Fatal(e)
		}
		messages, unsubscribe := f.shares.SubscribeXrayGroupMessages(1)
		defer unsubscribe()
		req := share.XrayGroupSendRequest{ServerID: "main", PlayerName: "Älex", Player: &importer.XrayPlayerRiskView{PlayerName: "Älex"}}
		f.request(t, "POST", "/api/share/xray/send-to-group", req, "admin", 500, nil)
		if e := f.db.QueryRowContext(ctx, `select count(*) from share_links`).Scan(&after); e != nil || after != before {
			t.Fatalf("failed enqueue left orphan share: before=%d after=%d error=%v", before, after, e)
		}
		select {
		case <-messages:
			t.Fatal("rolled back send was broadcast")
		default:
		}
		if _, e := f.db.ExecContext(ctx, `drop trigger reject_test_outbox`); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("unknown wrong-kind expired and cascading cleanup", func(t *testing.T) {
		prefixes := map[string]string{"player": "/api/share/", "ranking": "/api/share/ranking/", "log": "/api/share/log-query/", "xray": "/api/share/xray/"}
		for kind, token := range tokens {
			for other, prefix := range prefixes {
				if kind != other {
					f.request(t, "GET", prefix+token, nil, "", 404, nil)
				}
			}
			f.request(t, "GET", prefixes[kind]+"missing-token", nil, "", 404, nil)
		}
		past := time.Now().Add(-2 * time.Hour)
		if _, e := f.db.ExecContext(ctx, `update share_links set created_at=?,expires_at=?`, past, past.Add(time.Hour)); e != nil {
			t.Fatal(e)
		}
		for kind, token := range tokens {
			f.request(t, "GET", prefixes[kind]+token, nil, "", 410, nil)
		}
		if e := f.shares.CleanupExpiredTokens(ctx); e != nil {
			t.Fatal(e)
		}
		var n int
		if e := f.db.QueryRowContext(ctx, `select count(*) from share_pages`).Scan(&n); e != nil || n != 0 {
			t.Fatal("share pages not cascaded", n, e)
		}
		if e := f.db.Check(ctx); e != nil {
			t.Fatal(e)
		}
	})
}

func TestSQLitePublicPluginWebSocket(t *testing.T) {
	f := newPublicFixture(t)
	server := httptest.NewServer(f.handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/share/xray-group-messages/ws"
	if c, r, e := websocket.Dial(ctx, endpoint, nil); e == nil {
		c.CloseNow()
		t.Fatal("anonymous WebSocket accepted")
	} else if r == nil || r.StatusCode != 401 {
		t.Fatal("unexpected anonymous handshake", e)
	}
	req := share.XrayGroupSendRequest{ServerID: "main", PlayerName: "Älex", Player: &importer.XrayPlayerRiskView{PlayerName: "Älex"}}
	queued, e := f.shares.SendXrayToGroup(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	conn, _, e := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{settings.AstrBotAPIKeyHeader: []string{f.key}}})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	_, body, e := conn.Read(ctx)
	if e != nil {
		t.Fatal("pending replay failed", e)
	}
	var replay share.XrayGroupMessageResponse
	if e = json.Unmarshal(body, &replay); e != nil || replay.ID != queued.MessageID {
		t.Fatal("wrong replay", e)
	}
	type messageResult struct {
		body []byte
		err  error
	}
	liveMessage := make(chan messageResult, 1)
	go func() { _, b, e := conn.Read(ctx); liveMessage <- messageResult{b, e} }()
	// Reading must continue while Ping waits for the Pong control frame.
	// The round trip also lets the server install its subscriber after replay.
	if e = conn.Ping(ctx); e != nil {
		t.Fatal("control frame reader missing", e)
	}
	sent, e := f.shares.SendXrayToGroup(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	received := <-liveMessage
	body, e = received.body, received.err
	if e != nil {
		t.Fatal("live delivery failed", e)
	}
	var live share.XrayGroupMessageResponse
	if e = json.Unmarshal(body, &live); e != nil || live.ID != sent.MessageID {
		t.Fatal("wrong live message", e)
	}
	if e = conn.Close(websocket.StatusNormalClosure, ""); e != nil {
		t.Fatal("disconnect did not release stream", e)
	}
}

func TestSQLiteUnifiedPlayerSuggestionsAcrossSources(t *testing.T) {
	f := newPublicFixture(t)
	ctx := context.Background()
	e := f.db.Write(ctx, func(tx *storage.Tx) error {
		if _, e := tx.ExecContext(ctx, `insert into players(account_name,account_key) values('OtherÄlex',account_key('OtherÄlex'))`); e != nil {
			return e
		}
		_, e := tx.ExecContext(ctx, `insert into player_profiles(source_id,player_id,first_seen_at) select 'sub',id,? from players where account_key=account_key('OtherÄlex')`, time.Now())
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		scope string
		want  string
	}{{"main", "Älex"}, {"sub", "Älex,OtherÄlex"}, {"all", "Älex,OtherÄlex"}}
	for _, c := range cases {
		t.Run(c.scope, func(t *testing.T) {
			var names []string
			f.request(t, "GET", "/api/stats/player-suggestions?serverId="+c.scope+"&q=%C3%A4L", nil, "admin", 200, &names)
			if strings.Join(names, ",") != c.want {
				t.Fatalf("expected %s, got %v", c.want, names)
			}
		})
	}
	var empty []string
	f.request(t, "GET", "/api/stats/player-suggestions?serverId=all&q=", nil, "admin", 200, &empty)
	if len(empty) != 0 {
		t.Fatal("blank input returned candidates")
	}
	f.request(t, "GET", "/api/stats/player-suggestions?serverId=all&q=%25", nil, "admin", 200, &empty)
	if len(empty) != 0 {
		t.Fatal("wildcard was not literal")
	}
	f.request(t, "GET", "/api/stats/player-suggestions?serverId=all&q=a", nil, "", 401, nil)
	f.request(t, "GET", "/api/stats/player-suggestions?serverId=all&q=a", nil, "plugin", 401, nil)
}

func TestSQLiteSettingsOnlyUseCanonicalConfigurationRoutes(t *testing.T) {
	f := newPublicFixture(t)
	ctx := context.Background()
	// Removed routes cannot read or change configuration, even for an admin.
	paths := []string{"/api/config/sync", "/api/admin/group-chat/config", "/api/admin/group-chat/copy-schedule"}
	for _, path := range paths {
		for _, method := range []string{"GET", "PUT"} {
			f.request(t, method, path, map[string]any{"enabled": true, "skipToday": false}, "admin", 404, nil)
		}
	}
	for _, path := range []string{"/api/config/astrbot-key", "/api/config/source-files", "/api/admin/group-chat/directory-files"} {
		f.request(t, "GET", path, nil, "admin", 404, nil)
	}
	for _, path := range []string{"/api/config/astrbot-key/reset", "/api/config/smb-test"} {
		f.request(t, "POST", path, nil, "admin", 404, nil)
	}
	for _, section := range []string{"smb", "schedules", "sharing", "runtime"} {
		var current map[string]any
		f.request(t, "GET", "/api/admin/settings/"+section, nil, "admin", 200, &current)
		if _, old := current["autoRun"]; old {
			t.Fatal("mixed config field still exposed")
		}
		if _, old := current["syncCron"]; old {
			t.Fatal("cron compatibility field still exposed")
		}
		if section == "smb" {
			for _, source := range current["sources"].([]any) {
				if _, old := source.(map[string]any)["id"]; old {
					t.Fatal("synthetic source ID still exposed")
				}
			}
		}
		request := current
		if section == "runtime" {
			request = current["saved"].(map[string]any)
		}
		f.request(t, "PUT", "/api/admin/settings/"+section, request, "admin", 200, nil)
		request["autoRun"] = true
		f.request(t, "PUT", "/api/admin/settings/"+section, request, "admin", 400, nil)
	}
	f.request(t, "PUT", "/api/admin/settings/sharing", map[string]int{"shareTtlMinutes": 1, "logQueryShareTtlMinutes": 60}, "admin", 400, nil)
	var key settings.AstrBotKeyResponse
	f.request(t, "GET", "/api/admin/settings/plugin-key", nil, "admin", 200, &key)
	if key.APIKey != f.key {
		t.Fatal("removed endpoint changed plugin key")
	}
	f.request(t, "GET", "/api/admin/settings/plugin-key", nil, "plugin", 401, nil)
	f.request(t, "POST", "/api/admin/settings/plugin-key/reset", nil, "plugin", 401, nil)
	f.request(t, "POST", "/api/admin/settings/smb/test", nil, "plugin", 401, nil)
	f.request(t, "GET", "/api/admin/settings/smb/files?sourceId=main", nil, "plugin", 401, nil)
	f.request(t, "GET", "/api/admin/settings/smb/chat-files", nil, "plugin", 401, nil)
	var count int
	if e := f.db.QueryRowContext(ctx, `select count(*) from import_files`).Scan(&count); e != nil || count != 2 {
		t.Fatalf("settings cleanup changed imported data: %d %v", count, e)
	}
	var skip bool
	if e := f.db.QueryRowContext(ctx, `select skip_today from ingest_policy`).Scan(&skip); e != nil || !skip {
		t.Fatal("removed configuration endpoint changed date policy", e)
	}
}
