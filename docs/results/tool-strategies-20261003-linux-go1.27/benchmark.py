from pathlib import Path
import os,json,time,resource,subprocess,statistics,hashlib,shutil
ART=Path(__file__).resolve().parent
ROOT=Path('/var/tmp/dd-ci-testing-poc-20261001/minitracer/worktree')
GO=Path('/var/tmp/dd-ci-testing-poc-20261001/minitracer/go1.27.1/bin/go')
MODES=['current','find','one-pass']
BUDGET=24*1024**3
CASES={
 'plain':('plain',[]),
 'assert-only':('assert',[]),
 'suite-direct':('suite',[]),
 'suite-external-helper':('external',[]),
 'suite-client-coverage':('suite',['-coverpkg=./...','-covermode=atomic']),
 'suite-testing-coverage':('suite',['-coverpkg=testing,github.com/stretchr/testify/suite','-covermode=atomic']),
}
ENV=os.environ.copy()
ENV.update(PATH=str(GO.parent)+':'+ENV['PATH'],GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',TMPDIR=str(ART/'tmp'),DD_CIVISIBILITY_ENABLED='false',GOFLAGS='',GOCACHE='/var/tmp/dd-ci-testing-poc-20261001/minitracer/gocache')
ALLOWED=sorted(os.sched_getaffinity(0));assert len(ALLOWED)>=32
ROWS=[]
(ART/'matrix-final').mkdir(exist_ok=True)
(ART/'matrix-final'/'logs').mkdir(exist_ok=True)
(ART/'benchmark-fixtures').mkdir(exist_ok=True)
(ART/'matrix-final'/'caches').mkdir(exist_ok=True)
(ART/'matrix-final'/'outputs').mkdir(exist_ok=True)
EPOCH=time.time_ns()
base_source='package fixture\nfunc Add(a,b int)int{return a+b}\n'
for name,(kind,flags) in CASES.items():
 d=ART/'benchmark-fixtures'/name;d.mkdir(exist_ok=True)
 mod='module example.com/strategy-'+name+'\n\ngo 1.26.0\nrequire (\ngithub.com/tonyredondo/dd-ci-testing-poc v0.0.0\ngithub.com/stretchr/testify v1.11.1\n)\nreplace github.com/tonyredondo/dd-ci-testing-poc => '+str(ROOT)+'\n'
 if kind=='external':
  helper=d/'testkit';helper.mkdir(exist_ok=True)
  (helper/'go.mod').write_text('module example.com/testkit\n\ngo 1.26.0\nrequire github.com/stretchr/testify v1.11.1\n')
  (helper/'run.go').write_text('package testkit\nimport("testing";"github.com/stretchr/testify/suite")\ntype ExampleSuite struct{suite.Suite}\nfunc(s *ExampleSuite)TestPass(){s.True(true)}\nfunc Run(t *testing.T){suite.Run(t,new(ExampleSuite))}\n')
  mod+='require example.com/testkit v0.0.0\nreplace example.com/testkit => '+str(helper)+'\n'
 (d/'go.mod').write_text(mod)
 (d/'sample.go').write_text(base_source)
 source={
  'plain':'package fixture\nimport "testing"\nfunc TestPass(t *testing.T){if Add(2,3)!=5{t.Fatal("addition")}}\n',
  'assert':'package fixture\nimport("testing";"github.com/stretchr/testify/assert")\nfunc TestPass(t *testing.T){assert.Equal(t,5,Add(2,3))}\n',
  'suite':'package fixture\nimport("testing";"github.com/stretchr/testify/suite")\ntype ExampleSuite struct{suite.Suite}\nfunc(s *ExampleSuite)TestPass(){s.Equal(5,Add(2,3))}\nfunc TestSuite(t *testing.T){suite.Run(t,new(ExampleSuite))}\n',
  'external':'package fixture\nimport("testing";"example.com/testkit")\nfunc TestSuite(t *testing.T){testkit.Run(t)}\n',
 }[kind]
 (d/'sample_test.go').write_text(source)
 seeded=subprocess.run([str(GO),'list','-mod=mod','-deps','-test','.','testing','github.com/tonyredondo/dd-ci-testing-poc/testopt'],cwd=d,env=ENV,capture_output=True,text=True)
 assert seeded.returncode==0,seeded.stderr

def save():
 summary={}
 for r in ROWS:
  if r['phase']=='warmup':continue
  key=f"{r['case']}/{r['cores']}/{r['phase']}"
  summary.setdefault(key,{}).setdefault(r['mode'],[]).append(r)
 for modes in summary.values():
  for mode,samples in list(modes.items()):
   modes[mode]={k:statistics.median(r[k] for r in samples) for k in ['wall_ms','cpu_ms']}
   modes[mode].update(n=len(samples),wall_min_ms=min(r['wall_ms'] for r in samples),wall_max_ms=max(r['wall_ms'] for r in samples))
 data={'scope':'Mini CLI compile-only; same source and native fingerprints. Empty GOCACHE and removed output for each cold observation; actual compile and link required. Cached module downloads and filesystem. -ldflags=-w, -p and GOMAXPROCS equal CPU affinity 4/32. No test binaries executed. Balanced order; all observations retained.','go':subprocess.check_output([str(GO),'version'],env=ENV,text=True).strip(),'binaries':{m:hashlib.sha256((ART/('ddtest-'+m)).read_bytes()).hexdigest() for m in MODES},'summary':summary,'observations':ROWS}
 (ART/'benchmark-results.json').write_text(json.dumps(data,indent=2)+'\n')

def run(name,cores,phase,iteration,mode,cache):
 fixture=ART/'benchmark-fixtures'/name
 env=ENV.copy();env.update(GOMAXPROCS=str(cores),GOCACHE=str(cache))
 output_mode='shared' if phase=='switch-cache' else mode
 args=[str(ART/('ddtest-'+mode)),'test','--runtime=mini','-mod=mod','-p='+str(cores),'-ldflags=-w','-x','-c','-o',str(ART/'matrix-final'/'outputs'/f'{name}-{cores}-{output_mode}.test'),*CASES[name][1],'.']
 affinity=ALLOWED[:cores]
 if phase=='cold':
  assert not any(cache.iterdir()),str(cache)
  output=Path(args[args.index('-o')+1]);output.unlink(missing_ok=True)
  assert not output.exists()
 before=resource.getrusage(resource.RUSAGE_CHILDREN);started=time.perf_counter_ns()
 child=subprocess.run(args,cwd=fixture,env=env,capture_output=True,text=True,preexec_fn=lambda:os.sched_setaffinity(0,affinity),timeout=900)
 wall_ms=(time.perf_counter_ns()-started)/1e6;after=resource.getrusage(resource.RUSAGE_CHILDREN)
 label=f'{name}-{cores}-{phase}-{iteration}-{mode}'
 log=ART/'matrix-final'/'logs'/(label+'.log');log.write_text(child.stdout+child.stderr)
 if child.returncode:raise RuntimeError(label+'\n'+child.stdout+child.stderr[-7000:])
 tools={tool:sum(1 for line in child.stderr.splitlines() if '/'+tool+' ' in line and '-V=full' not in line) for tool in ['compile','asm','link','cover']}
 if phase=='cold':assert tools['compile']>0 and tools['link']>0,('cold skipped compile/link',label,tools)
 if phase=='edit':assert tools['compile']>0,('edit reused cached objects',label)
 if phase=='unchanged':assert tools['compile']==tools['link']==0,('unchanged rebuilt',label,tools)
 row={'case':name,'cores':cores,'phase':phase,'iteration':iteration,'mode':mode,'wall_ms':wall_ms,'cpu_ms':(after.ru_utime+after.ru_stime-before.ru_utime-before.ru_stime)*1000,'tools':tools,'wrapper':'tool-overlay' in child.stderr,'source_sha256':hashlib.sha256((fixture/'sample.go').read_bytes()).hexdigest(),'gomod_sha256':hashlib.sha256((fixture/'go.mod').read_bytes()).hexdigest(),'affinity':affinity,'command':args,'log':str(log),'loadavg':Path('/proc/loadavg').read_text().strip()}
 ROWS.append(row);save();print(json.dumps({k:row[k] for k in ['case','cores','phase','iteration','mode','wall_ms','cpu_ms','tools','wrapper']}),flush=True)
 return row

for cores in [4,32]:
 for name in CASES:
  size=int(subprocess.check_output(['du','-sb',str(ART)],text=True).split()[0]);assert size<BUDGET,size
  keep={}
  for i in range(3):
   order=MODES[i:]+MODES[:i]
   for mode in order:
    cache=ART/'matrix-final'/'caches'/f'{name}-{cores}-{mode}-{i}';cache.mkdir()
    run(name,cores,'cold',i,mode,cache)
    if i<2:shutil.rmtree(cache) # Only this completed observation's regenerable cache.
    else:keep[mode]=cache
  for i in range(7):
   order=MODES[i%3:]+MODES[:i%3]
   for mode in order:run(name,cores,'unchanged',i,mode,keep[mode])
  fixture=ART/'benchmark-fixtures'/name
  try:
   for i in range(7):
    order=MODES[(i+1)%3:]+MODES[:(i+1)%3]
    for mode in order:
     (fixture/'sample.go').write_text(base_source+f'\nconst EditedInput = {EPOCH+len(ROWS)}\n')
     run(name,cores,'edit',i,mode,keep[mode])
  finally:(fixture/'sample.go').write_text(base_source)
  # Restore a warm unchanged artifact after the edit sequence, then verify
  # switching modes against one shared cache preserves native package keys.
  shared=keep['current']
  for i,mode in enumerate(['current','find','one-pass','current']):
   row=run(name,cores,'switch-cache',i,mode,shared)
   if i>0:assert row['tools']['compile']==row['tools']['link']==0,('cache switch rebuilt',row)
   size=int(subprocess.check_output(['du','-sb',str(ART)],text=True).split()[0]);assert size<BUDGET,size
print('COMPLETE',len(ROWS),'observations',flush=True)
