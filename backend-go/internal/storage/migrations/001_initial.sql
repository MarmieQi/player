CREATE TABLE database_metadata(id INTEGER PRIMARY KEY CHECK(id=1), instance_id TEXT NOT NULL, rebuild_required INTEGER NOT NULL DEFAULT 0 CHECK(rebuild_required IN(0,1))) STRICT;
CREATE TABLE admin_users(id INTEGER PRIMARY KEY, username TEXT NOT NULL, username_key TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, enabled INTEGER NOT NULL CHECK(enabled IN(0,1)),created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL) STRICT;
CREATE TABLE smb_connections(id INTEGER PRIMARY KEY CHECK(id=1),host TEXT NOT NULL DEFAULT '',port INTEGER NOT NULL DEFAULT 445 CHECK(port BETWEEN 1 AND 65535),domain TEXT NOT NULL DEFAULT '',username TEXT NOT NULL DEFAULT '',password TEXT NOT NULL DEFAULT '',share TEXT NOT NULL DEFAULT '') STRICT;
CREATE TABLE log_sources(source_id TEXT PRIMARY KEY,source_name TEXT NOT NULL,kind TEXT NOT NULL CHECK(kind IN('csv','txt')),local_subdirectory TEXT NOT NULL UNIQUE,connection_id INTEGER NOT NULL DEFAULT 1 REFERENCES smb_connections(id),smb_directory TEXT NOT NULL DEFAULT '',smb_file_glob TEXT NOT NULL,smb_recursive INTEGER NOT NULL DEFAULT 0 CHECK(smb_recursive IN(0,1)),enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN(0,1)),CHECK(kind='csv' OR smb_recursive=0)) STRICT;
CREATE TABLE ingest_policy(id INTEGER PRIMARY KEY CHECK(id=1),skip_today INTEGER NOT NULL DEFAULT 1 CHECK(skip_today IN(0,1))) STRICT;
CREATE TABLE sharing_settings(id INTEGER PRIMARY KEY CHECK(id=1),astrbot_api_key TEXT NOT NULL,share_ttl_minutes INTEGER NOT NULL DEFAULT 60 CHECK(share_ttl_minutes BETWEEN 5 AND 10080),log_query_share_ttl_minutes INTEGER NOT NULL DEFAULT 60 CHECK(log_query_share_ttl_minutes BETWEEN 5 AND 10080)) STRICT;
CREATE TABLE runtime_settings(id INTEGER PRIMARY KEY CHECK(id=1),live_cache_seconds INTEGER NOT NULL CHECK(live_cache_seconds BETWEEN 10 AND 600),max_files_per_run INTEGER NOT NULL CHECK(max_files_per_run BETWEEN 1 AND 100000),xray_retained_jobs INTEGER NOT NULL CHECK(xray_retained_jobs BETWEEN 1 AND 20)) STRICT;
CREATE TABLE task_schedules(id INTEGER PRIMARY KEY,source_id TEXT NOT NULL REFERENCES log_sources(source_id),kind TEXT NOT NULL CHECK(kind IN('SYNC','IMPORT')),enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN(0,1)),run_time TEXT NOT NULL CHECK(length(run_time)=5 AND run_time GLOB '[0-2][0-9]:[0-5][0-9]' AND substr(run_time,1,2)<'24'),UNIQUE(source_id,kind)) STRICT;
CREATE TRIGGER schedule_txt_guard BEFORE INSERT ON task_schedules WHEN NEW.kind='IMPORT' AND (SELECT kind FROM log_sources WHERE source_id=NEW.source_id)='txt' BEGIN SELECT RAISE(ABORT,'TXT sources cannot be parsed'); END;
CREATE TRIGGER schedule_txt_update_guard BEFORE UPDATE ON task_schedules WHEN NEW.kind='IMPORT' AND (SELECT kind FROM log_sources WHERE source_id=NEW.source_id)='txt' BEGIN SELECT RAISE(ABORT,'TXT sources cannot be parsed'); END;
CREATE TABLE task_runs(id INTEGER PRIMARY KEY,schedule_id INTEGER REFERENCES task_schedules(id),run_date TEXT,run_time TEXT,created_at INTEGER NOT NULL,finished_at INTEGER,hidden INTEGER NOT NULL DEFAULT 0 CHECK(hidden IN(0,1)),server_id TEXT NOT NULL REFERENCES log_sources(source_id),server_name TEXT NOT NULL,task_type TEXT NOT NULL,task_label TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN('RUNNING','FINISHED','FINISHED_WITH_ERRORS','FAILED','INTERRUPTED')),message TEXT NOT NULL DEFAULT '',file_details TEXT NOT NULL DEFAULT '',scanned_files INTEGER NOT NULL DEFAULT 0 CHECK(scanned_files>=0),success_files INTEGER NOT NULL DEFAULT 0 CHECK(success_files>=0),skipped_files INTEGER NOT NULL DEFAULT 0 CHECK(skipped_files>=0),failed_files INTEGER NOT NULL DEFAULT 0 CHECK(failed_files>=0),UNIQUE(schedule_id,run_date,run_time)) STRICT;
CREATE INDEX task_runs_created ON task_runs(created_at);
CREATE TABLE players(id INTEGER PRIMARY KEY,account_key TEXT NOT NULL UNIQUE,account_name TEXT NOT NULL) STRICT;
CREATE TABLE import_files(id INTEGER PRIMARY KEY,source_id TEXT NOT NULL REFERENCES log_sources(source_id),relative_path TEXT NOT NULL CHECK(relative_path<>'' AND instr(relative_path,'/')=0 AND instr(relative_path,char(92))=0 AND relative_path NOT IN('.','..')),log_date TEXT,file_size INTEGER NOT NULL CHECK(file_size>=0),last_modified INTEGER NOT NULL,content_hash TEXT NOT NULL,parser_version INTEGER NOT NULL,imported_at INTEGER NOT NULL,row_count INTEGER NOT NULL CHECK(row_count>=0),ignored_count INTEGER NOT NULL CHECK(ignored_count>=0),UNIQUE(source_id,relative_path)) STRICT;
CREATE INDEX import_files_source_imported ON import_files(source_id,imported_at);
CREATE INDEX import_files_date ON import_files(log_date);
CREATE TABLE file_player_daily(file_id INTEGER NOT NULL REFERENCES import_files(id) ON DELETE CASCADE,player_id INTEGER NOT NULL REFERENCES players(id),stat_date TEXT NOT NULL,broken_count INTEGER NOT NULL CHECK(broken_count>=0),placed_count INTEGER NOT NULL CHECK(placed_count>=0),PRIMARY KEY(file_id,player_id,stat_date)) STRICT,WITHOUT ROWID;
CREATE INDEX file_daily_player_date ON file_player_daily(player_id,stat_date);
CREATE TABLE file_player_presence(file_id INTEGER NOT NULL REFERENCES import_files(id) ON DELETE CASCADE,player_id INTEGER NOT NULL REFERENCES players(id),first_seen_at INTEGER NOT NULL,PRIMARY KEY(file_id,player_id)) STRICT,WITHOUT ROWID;
CREATE INDEX file_presence_player ON file_player_presence(player_id,first_seen_at);
CREATE TABLE file_player_resources(file_id INTEGER NOT NULL REFERENCES import_files(id) ON DELETE CASCADE,player_id INTEGER NOT NULL REFERENCES players(id),category TEXT NOT NULL CHECK(category IN('ore','wood','sapling')),resource_type TEXT NOT NULL,resource_count INTEGER NOT NULL CHECK(resource_count>=0),PRIMARY KEY(file_id,player_id,category,resource_type)) STRICT,WITHOUT ROWID;
CREATE INDEX file_resources_player ON file_player_resources(player_id,category,resource_type);
CREATE TABLE file_player_milestones(file_id INTEGER NOT NULL REFERENCES import_files(id) ON DELETE CASCADE,player_id INTEGER NOT NULL REFERENCES players(id),milestone_type TEXT NOT NULL,first_seen_at INTEGER NOT NULL,detail TEXT NOT NULL DEFAULT '',PRIMARY KEY(file_id,player_id,milestone_type)) STRICT,WITHOUT ROWID;
CREATE INDEX file_milestones_player ON file_player_milestones(player_id,milestone_type);
CREATE TABLE player_daily_stats(source_id TEXT NOT NULL REFERENCES log_sources(source_id),player_id INTEGER NOT NULL REFERENCES players(id),stat_date TEXT NOT NULL,broken_count INTEGER NOT NULL CHECK(broken_count>=0),placed_count INTEGER NOT NULL CHECK(placed_count>=0),PRIMARY KEY(source_id,player_id,stat_date)) STRICT,WITHOUT ROWID;
CREATE INDEX daily_date_player ON player_daily_stats(stat_date,player_id);
CREATE INDEX daily_source_date ON player_daily_stats(source_id,stat_date);
CREATE TABLE player_profiles(source_id TEXT NOT NULL REFERENCES log_sources(source_id),player_id INTEGER NOT NULL REFERENCES players(id),first_seen_at INTEGER NOT NULL,PRIMARY KEY(source_id,player_id)) STRICT,WITHOUT ROWID;
CREATE INDEX profiles_player ON player_profiles(player_id,first_seen_at);
CREATE TABLE share_links(token TEXT PRIMARY KEY,kind TEXT NOT NULL CHECK(kind IN('player','ranking','log','xray')),player_name TEXT,server_id TEXT,ranking_type TEXT,limit_count INTEGER,from_date TEXT,to_date TEXT,created_at INTEGER NOT NULL,expires_at INTEGER NOT NULL CHECK(expires_at>=created_at),payload_json TEXT CHECK(payload_json IS NULL OR json_valid(payload_json))) STRICT;
CREATE INDEX shares_expires ON share_links(expires_at);
CREATE TABLE share_pages(token TEXT NOT NULL REFERENCES share_links(token) ON DELETE CASCADE,page_number INTEGER NOT NULL CHECK(page_number>0),row_count INTEGER NOT NULL CHECK(row_count>=0),payload_gzip BLOB NOT NULL,PRIMARY KEY(token,page_number)) STRICT,WITHOUT ROWID;
CREATE TABLE xray_analysis_jobs (
job_id TEXT NOT NULL,
server_id TEXT NOT NULL,
server_name TEXT NOT NULL,
status TEXT NOT NULL CHECK(status IN('PENDING','RUNNING','FINISHED','FINISHED_WITH_ERRORS','FAILED','CANCELLED')),
started_at INTEGER NOT NULL,
finished_at INTEGER,
from_time INTEGER,
to_time INTEGER,
player_name TEXT,
dimension TEXT,
scanned_files INTEGER NOT NULL,
scanned_rows INTEGER NOT NULL,
failed_files INTEGER NOT NULL,
player_count INTEGER NOT NULL,
finding_count INTEGER NOT NULL,
max_risk_score INTEGER NOT NULL,
message TEXT,
payload_json TEXT NOT NULL CHECK(json_valid(payload_json)),
PRIMARY KEY (job_id)
) STRICT;
CREATE TABLE notification_outbox(id INTEGER PRIMARY KEY,status TEXT NOT NULL CHECK(status IN('PENDING','SENT','FAILED')),created_at INTEGER NOT NULL,expires_at INTEGER NOT NULL,delivered_at INTEGER,failure_message TEXT NOT NULL DEFAULT '',payload_json TEXT NOT NULL CHECK(json_valid(payload_json))) STRICT;
CREATE INDEX xray_jobs_source_started ON xray_analysis_jobs(server_id,started_at);
CREATE INDEX outbox_status_created ON notification_outbox(status,created_at);
CREATE VIEW v_import_files AS SELECT f.id,f.source_id AS server_id,s.source_name AS server_name,f.relative_path AS remote_path,f.relative_path AS file_name,f.log_date,f.file_size,f.last_modified,f.content_hash,f.imported_at,f.row_count,f.ignored_count,f.parser_version FROM import_files f JOIN log_sources s ON s.source_id=f.source_id;
CREATE VIEW v_daily_stats AS SELECT d.source_id AS server_id,s.source_name AS server_name,d.stat_date,p.account_name AS player_name,d.broken_count,d.placed_count FROM player_daily_stats d JOIN players p ON p.id=d.player_id JOIN log_sources s ON s.source_id=d.source_id;
CREATE VIEW v_profiles AS SELECT d.source_id AS server_id,s.source_name AS server_name,p.account_name AS player_name,p.account_key AS player_key,d.first_seen_at FROM player_profiles d JOIN players p ON p.id=d.player_id JOIN log_sources s ON s.source_id=d.source_id;
CREATE VIEW v_file_daily AS SELECT d.file_id AS import_file_id,p.account_name AS player_name,d.stat_date,d.broken_count,d.placed_count FROM file_player_daily d JOIN players p ON p.id=d.player_id;
CREATE VIEW v_file_presence AS SELECT d.file_id AS import_file_id,p.account_name AS player_name,d.first_seen_at FROM file_player_presence d JOIN players p ON p.id=d.player_id;
CREATE VIEW v_file_milestones AS SELECT d.file_id AS import_file_id,p.account_name AS player_name,d.milestone_type,d.first_seen_at,d.detail FROM file_player_milestones d JOIN players p ON p.id=d.player_id;
CREATE VIEW v_file_ore AS SELECT d.file_id AS import_file_id,p.account_name AS player_name,d.resource_type AS ore_type,d.resource_count AS ore_count FROM file_player_resources d JOIN players p ON p.id=d.player_id WHERE d.category='ore';
CREATE VIEW v_file_wood AS SELECT d.file_id AS import_file_id,p.account_name AS player_name,d.resource_type AS wood_type,d.resource_count AS wood_count FROM file_player_resources d JOIN players p ON p.id=d.player_id WHERE d.category='wood';
CREATE VIEW v_file_sapling AS SELECT d.file_id AS import_file_id,p.account_name AS player_name,d.resource_type AS sapling_type,d.resource_count AS sapling_count FROM file_player_resources d JOIN players p ON p.id=d.player_id WHERE d.category='sapling';
CREATE VIEW v_player_shares AS SELECT * FROM share_links WHERE kind='player';
CREATE VIEW v_ranking_shares AS SELECT * FROM share_links WHERE kind='ranking';
CREATE VIEW v_log_query_shares AS SELECT * FROM share_links WHERE kind='log';
CREATE VIEW v_xray_shares AS SELECT * FROM share_links WHERE kind='xray';
