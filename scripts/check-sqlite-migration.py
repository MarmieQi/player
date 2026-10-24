"""Exercise cutover ordering and failure recovery with isolated command stubs."""
from pathlib import Path
import os,subprocess,tempfile
import shutil
root=Path(__file__).resolve().parents[1]
if os.name=='nt':
 bash=str(Path(shutil.which('git')).resolve().parent.parent/'bin/bash.exe')
else:
 bash=shutil.which('sh')
sha='a'*40
for mode in ('success','pullfail','migratefail','rebuildfail','resume'):
 with tempfile.TemporaryDirectory(prefix='player-migration-script-') as temp:
  work=Path(temp);bins=work/'bin';bins.mkdir();log=work/'calls.log'
  (work/'.env').write_text('IMAGE_TAG=old\nMYSQL_PASSWORD=fixture\n')
  (work/'docker-compose.yml').write_text('old-compose')
  docker='''#!/bin/sh
printf '%s\\n' "$*" >> "$FIXTURE_LOG"
case "$*" in
'compose config --services') printf 'mysql\\nbackend\\nfrontend\\n';;
'inspect -f {{.State.Running}} player-stats-mysql') echo true;;
inspect*) echo player_default;;
*'pull backend frontend') test "$FIXTURE_MODE" != pullfail;;
*'run --rm config-migrate') test "$FIXTURE_MODE" != migratefail;;
*'backend rebuild') test "$FIXTURE_MODE" != rebuildfail;;
*) exit 0;;
esac
'''
  curl='''#!/bin/sh
while [ "$#" -gt 0 ]; do
 if [ "$1" = -o ]; then shift; printf 'downloaded-compose' > "$1"; exit 0; fi
 shift
done
exit 1
'''
  (bins/'docker').write_text(docker,newline='\n');(bins/'curl').write_text(curl,newline='\n')
  (bins/'docker').chmod(0o755);(bins/'curl').chmod(0o755)
  env=os.environ.copy();env['PATH']=str(bins)+os.pathsep+env['PATH'];env['FIXTURE_LOG']=str(log).replace('\\','/');env['FIXTURE_MODE']=mode
  env.pop('MIGRATION_NETWORK',None)
  env['FIXTURE_BIN']=('/'+str(bins)[0].lower()+str(bins)[2:].replace('\\','/')) if os.name=='nt' else str(bins)
  env['FIXTURE_SCRIPT']=str(root/'scripts/migrate-sqlite.sh').replace('\\','/')
  cmd=[bash,'-c','export PATH="$FIXTURE_BIN:$PATH"; exec sh "$FIXTURE_SCRIPT" "$@"','fixture',sha]
  if mode=='resume':cmd+=['--resume']
  p=subprocess.run(cmd,cwd=work,env=env,capture_output=True,text=True)
  calls=log.read_text() if log.exists() else ''
  if mode in ('success','resume'):
   assert p.returncode==0,(mode,p.stdout,p.stderr,calls)
   assert (work/'docker-compose.mysql.yml').read_text()=='old-compose'
   assert 'IMAGE_TAG='+sha in (work/'.env').read_text()
  else:
   assert p.returncode!=0,(mode,p.stdout,p.stderr)
   assert (work/'docker-compose.yml').read_text()=='old-compose'
   assert not (work/'docker-compose.mysql.yml').exists()
  if mode=='pullfail':assert 'stop frontend backend' not in calls
  if mode=='resume':assert 'run --rm config-migrate' not in calls
  if mode=='migratefail':assert 'backend rebuild' not in calls
  if 'stop frontend backend' in calls:assert calls.index('pull config-migrate') < calls.index('stop frontend backend')
  print(mode,': passed')
