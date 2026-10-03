from pathlib import Path
import hashlib,json,os,resource,statistics,subprocess,time
art=Path('/var/tmp/dd-ci-testing-poc-selective-tools-20261003');root=Path.cwd()
env=os.environ.copy();env.update(PATH='/var/tmp/dd-ci-testing-poc-20261001/minitracer/go1.27.1/bin:'+env['PATH'],GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOMAXPROCS='4',GOCACHE='/var/tmp/dd-ci-testing-poc-20261001/minitracer/gocache',TMPDIR=str(art/'tmp'),DD_CIVISIBILITY_ENABLED='false',GOFLAGS='')
fixture=art/'perf-fixture';fixture.mkdir(exist_ok=True)
(fixture/'go.mod').write_text('module example.com/selective-tools-perf\n\ngo 1.26.0\nrequire (\ngithub.com/tonyredondo/dd-ci-testing-poc v0.0.0\ngithub.com/stretchr/testify v1.11.1\n)\nreplace github.com/tonyredondo/dd-ci-testing-poc => '+str(root)+'\n')
source=fixture/'sample.go';source.write_text('package fixture\nfunc Add(a,b int)int{return a+b}\n')
(fixture/'sample_test.go').write_text('package fixture\nimport "testing"\nfunc TestPass(t *testing.T){if Add(2,3)!=5{t.Fatal("addition")}}\n')
(fixture/'suite_test.go').write_text('//go:build suite\n\npackage fixture\nimport("testing";"github.com/stretchr/testify/suite")\ntype HTTPSuite struct{suite.Suite}\nfunc(s *HTTPSuite)TestPass(){s.True(true)}\nfunc TestSuite(t *testing.T){suite.Run(t,new(HTTPSuite))}\n')
rows=[]
edit_epoch=time.time_ns()//10000
def run(scope,phase,i,variant,args,cwd=fixture):
 before=resource.getrusage(resource.RUSAGE_CHILDREN);start=time.perf_counter_ns()
 child=subprocess.run(args,cwd=cwd,env=env,capture_output=True,text=True,timeout=90)
 elapsed=time.perf_counter_ns()-start;after=resource.getrusage(resource.RUSAGE_CHILDREN)
 if child.returncode:raise RuntimeError(child.stdout+child.stderr)
 tools={name:sum(1 for line in child.stderr.splitlines() if ('/'+name+' ' in line or '\\'+name+'.exe ' in line) and '-V=full' not in line) for name in ('compile','cover','link')}
 if phase=='edit' and tools['compile']==0:raise RuntimeError('Edit recovered cached objects instead of compiling: '+scope+' '+variant)
 row=dict(scope=scope,phase=phase,iteration=i,variant=variant,wall_ns=elapsed,cpu_ns=round((after.ru_utime+after.ru_stime-before.ru_utime-before.ru_stime)*1e9),tools=tools,wrapper='tool-overlay' in child.stderr or 'cover-overlay' in child.stderr,command=args)
 rows.append(row)
 print(scope,phase,i,variant,round(elapsed/1e6,2),flush=True)
# Warm dependencies without ever executing the test binaries.
for scope,flags in [('no-suite',[]),('testify',['-tags=suite']),('coverage',['-coverpkg=./...']),('testify-coverage',['-tags=suite','-coverpkg=./...'])]:
 cmds={variant:[str(art/('ddtest-'+variant)),'test','--runtime=mini','-mod=mod','-x','-c','-o',str(fixture/(scope+'-'+variant+'.test')),*flags,'.'] for variant in ('before','after')}
 for variant in cmds:run(scope,'warmup',0,variant,cmds[variant])
 for i in range(9):
  for variant in (('before','after') if i%2==0 else ('after','before')):run(scope,'unchanged',i,variant,cmds[variant])
 original=source.read_bytes()
 try:
  for i in range(7):
   for variant in (('after','before') if i%2==0 else ('before','after')):
    source.write_bytes(original+f'\nconst PerformanceInput = {edit_epoch*1000+len(rows)*2+(variant=="after")}\n'.encode())
    run(scope,'edit',i,variant,cmds[variant])
 finally:source.write_bytes(original)
 for i in range(3):
  for variant in (('before','after') if i%2==0 else ('after','before')):
   args=cmds[variant][:];args.insert(3,'-a');run(scope,'forced-rebuild',i,variant,args)
 (art/'measure-progress.json').write_text(json.dumps(rows,indent=2)+'\n')
# Version probe isolates wrapper process/dispatch cost from prepare/compile/link.
tooldir=subprocess.check_output(['go','env','GOTOOLDIR'],env=env,text=True).strip();compile=str(Path(tooldir)/'compile')
commands={'native':[compile,'-V=full'],'before':[str(art/'ddtest-before'),'cover-overlay','missing-plan',compile,'-V=full'],'after':[str(art/'ddtest-after'),'tool-overlay','testify','missing-plan',compile,'-V=full']}
for i in range(41):
 for variant in (('native','before','after') if i%2==0 else ('after','before','native')):run('bypass','probe',i,variant,commands[variant])
summary={}
for row in rows:
 if row['phase']=='warmup':continue
 key=row['scope']+'/'+row['phase'];summary.setdefault(key,{})
for key in summary:
 scope,phase=key.split('/')
 for variant in sorted({r['variant'] for r in rows if r['scope']==scope and r['phase']==phase}):
  sample=[r for r in rows if r['scope']==scope and r['phase']==phase and r['variant']==variant]
  summary[key][variant]={metric:statistics.median(r[field] for r in sample)/1e6 for metric,field in [('wall_median_ms','wall_ns'),('cpu_median_ms','cpu_ns')]}
  summary[key][variant].update(wall_min_ms=min(r['wall_ns'] for r in sample)/1e6,wall_max_ms=max(r['wall_ns'] for r in sample)/1e6,n=len(sample))
result=dict(scope='Compile-only CLI before/after, 4 GOMAXPROCS, stable output per variant, balanced ordering. Forced rebuild uses -a and warm module downloads; it is not an empty-cache benchmark. Probe measures only compile -V=full invocation. No test binaries executed.',go=subprocess.check_output(['go','version'],env=env,text=True).strip(),cores=4,binaries={v:hashlib.sha256((art/('ddtest-'+v)).read_bytes()).hexdigest() for v in ('before','after')},observations=rows,summary=summary)
(art/'performance.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(summary,indent=2),flush=True)
