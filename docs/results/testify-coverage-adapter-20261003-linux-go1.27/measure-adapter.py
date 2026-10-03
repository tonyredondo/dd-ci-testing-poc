from pathlib import Path
import hashlib,json,os,resource,statistics,subprocess,time
art=Path('/var/tmp/dd-ci-testing-poc-testify-analysis-20261002');fixture=art/'adapter-perf-fixture'
plan=json.loads((art/'adapter-perf-plan.json').read_text());env=os.environ.copy()
env.update(PATH='/var/tmp/dd-ci-testing-poc-20261001/minitracer/go1.27.1/bin:'+env['PATH'],GOTOOLCHAIN='local',GOPROXY='off',GOSUMDB='off',GOMAXPROCS='4',GOCACHE='/var/tmp/dd-ci-testing-poc-20261001/minitracer/gocache',TMPDIR=str(art/'tmp'),DD_CIVISIBILITY_ENABLED='false')
base=['go','test','-x','-mod=mod','-coverpkg=.','-overlay='+plan['File'],'-c','-o',str(fixture/'fixture.test'),'.']
bridge="'"+str(art/'ddtest')+"' cover-overlay '"+plan['File']+"'"
results=[]
def run(phase,iteration,mode):
 args=base[:]
 args[args.index("-o")+1]=str(fixture/("fixture-"+mode+".test"))
 if phase=='rebuild-all':args.insert(2,'-a')
 if mode=='adapter':args.insert(2,'-toolexec='+bridge)
 previous=resource.getrusage(resource.RUSAGE_CHILDREN);start=time.perf_counter_ns()
 child=subprocess.run(args,cwd=fixture,env=env,capture_output=True,text=True,timeout=90)
 elapsed=time.perf_counter_ns()-start;usage=resource.getrusage(resource.RUSAGE_CHILDREN)
 if child.returncode:raise RuntimeError(child.stdout+child.stderr)
 tools={name:sum(1 for line in child.stderr.splitlines() if ('/'+name+' ' in line or '\\'+name+'.exe ' in line)) for name in ('compile','cover','link')}
 row=dict(phase=phase,iteration=iteration,mode=mode,tool_invocations=tools,wall_ns=elapsed,user_s=usage.ru_utime-previous.ru_utime,sys_s=usage.ru_stime-previous.ru_stime,command=args)
 results.append(row)
 print(phase,iteration,mode,round(elapsed/1e6,2),flush=True)
# Warm each separate cover-tool identity before measuring either variant.
for mode in ('direct','adapter'):run('warmup',0,mode)
for i in range(6):
 for mode in (('direct','adapter') if i%2==0 else ('adapter','direct')):run('unchanged',i,mode)
source=fixture/'sample.go';original=source.read_bytes()
try:
 for i in range(6):
  for mode in (('adapter','direct') if i%2==0 else ('direct','adapter')):
   # An unused exported constant forces compilation/linking, with unchanged test behavior.
   marker=f'\nconst DDTestPerformanceInput = {16000+i*2+(0 if mode=="direct" else 1)}\n'
   source.write_bytes(original+marker.encode())
   run('source-edit',i,mode)
finally:source.write_bytes(original)
for i in range(4):
 for mode in (('direct','adapter') if i%2==0 else ('adapter','direct')):run('rebuild-all',i,mode)
summary={}
for phase in ('unchanged','source-edit','rebuild-all'):
 summary[phase]={}
 for mode in ('direct','adapter'):
  rows=[r for r in results if r['phase']==phase and r['mode']==mode]
  summary[phase][mode]=dict(wall_median_ms=statistics.median(r['wall_ns'] for r in rows)/1e6,cpu_median_ms=statistics.median(r['user_s']+r['sys_s'] for r in rows)*1000,wall_min_ms=min(r['wall_ns'] for r in rows)/1e6,wall_max_ms=max(r['wall_ns'] for r in rows)/1e6)
 summary[phase]['delta_wall_ms']=summary[phase]['adapter']['wall_median_ms']-summary[phase]['direct']['wall_median_ms']
report=dict(output_policy='Separate stable output files per variant to prevent cross-variant relinking.',scope='Compile-only prepared Mini overlay, native coverage without rewritten normal callers; force the adapter to isolate process/version overhead. Preparation and SDK runtime execution excluded. Six balanced pairs after warmup for unchanged/edit builds, four -a pairs. Edit pairs add an unused exported constant with equal-length values to force comparable compiler/linker work.',go=subprocess.check_output(['go','version'],env=env,text=True).strip(),cores=4,driver_sha256=hashlib.sha256((art/'ddtest').read_bytes()).hexdigest(),plan=plan,observations=results,summary=summary)
(art/'adapter-overhead.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(summary,indent=2))
