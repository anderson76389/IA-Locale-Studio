"""Run the compiled Studio, Ollama and ComfyUI together on loopback."""
import argparse, contextlib, json, os, pathlib, shutil, subprocess, sys, tempfile, time
import urllib.request, urllib.error
import numpy as np
import av
from PIL import Image

parser=argparse.ArgumentParser()
for name in ("studio","ollama","comfy","models"):
    parser.add_argument("--"+name,type=pathlib.Path,required=True)
args=parser.parse_args()
models=args.models.resolve()
comfy=args.comfy.resolve()
ollama=str(args.ollama.resolve())
opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))

def request(url,body=None,token=None):
    headers={"Content-Type":"application/json"}
    if token: headers["X-Studio-Token"]=token
    req=urllib.request.Request(url,None if body is None else json.dumps(body).encode(),headers)
    with opener.open(req,timeout=90) as r:return json.load(r)

def ready(url,process):
    for _ in range(120):
        if process.poll() is not None:raise RuntimeError(f"Process exited {process.returncode}: {url}")
        try:return request(url)
        except (OSError,ValueError):time.sleep(1)
    raise TimeoutError(url)

def node(kind,**inputs):
    return {"class_type":kind,"inputs":inputs}

def run():
    env=dict(os.environ,STUDIO_TEST_MODELS=str(models),OLLAMA_HOST="127.0.0.1:11434",OLLAMA_MODELS=str(models/"ollama-models"),HF_HUB_OFFLINE="1",OMP_NUM_THREADS="2")
    path=comfy/"custom_nodes"/"studio_validation"
    path.mkdir(exist_ok=True)
    shutil.copyfile(pathlib.Path(__file__).with_name("comfy_models.py"),path/"__init__.py")
    with tempfile.TemporaryDirectory() as tmp,contextlib.ExitStack() as stack:
        folder=pathlib.Path(tmp)
        def start(cmd,log):
            stream=stack.enter_context(open(log,"w",encoding="utf-8"))
            proc=subprocess.Popen(cmd,env=env,stdout=stream,stderr=subprocess.STDOUT)
            def close():
                proc.terminate()
                try:proc.wait(10)
                except subprocess.TimeoutExpired:proc.kill();proc.wait()
            stack.callback(close)
            return proc
        engine=start([ollama,"serve"],"ollama-validation.log")
        ready("http://127.0.0.1:11434/api/tags",engine)
        modelfile=models/"Modelfile"
        modelfile.write_text("FROM ./text/SmolLM2-135M-Instruct-Q4_K_M.gguf\n",encoding="utf-8")
        subprocess.run([ollama,"create","studio-test","-f",str(modelfile)],cwd=models,env=env,check=True)
        engine=start([sys.executable,str(comfy/"main.py"),"--cpu","--listen","127.0.0.1","--port","8188"],"comfy-validation.log")
        ready("http://127.0.0.1:8188/system_stats",engine)
        studio=start([str(args.studio.resolve()),"-no-browser","-root",str(folder),"-port","14442"],"studio-validation.log")
        api="http://127.0.0.1:14442/api/"
        token=ready(api+"session",studio)["token"]
        for kind,prompt in [("texte","Name a planet."),("code","Write a Python function adding two numbers."),("image","a lighthouse beside the sea"),("video","waves under a blue sky"),("musique","an upbeat piano melody")]:
            q={"kind":kind,"prompt":prompt,"model":"studio-test:latest","context":2048,"tokens":64,"trusted":True}
            if kind in ("image","video"):
                wf={"1":node("StudioTestIllustration",prompt="{{PROMPT}}",frames=1 if kind=="image" else 4)}
                if kind=="image":wf["2"]=node("SaveImage",images=["1",0],filename_prefix="studio-image")
                else:
                    wf["2"]=node("CreateVideo",images=["1",0],fps=4)
                    wf["3"]=node("SaveVideo",video=["2",0],filename_prefix="studio-video",format="mp4",codec="h264")
                q["workflow"]=wf
            elif kind=="musique":
                q["workflow"]={"1":node("StudioTestMelody",prompt="{{PROMPT}}"),"2":node("SaveAudio",audio=["1",0],filename_prefix="studio-music")}
            job=request(api+"generate",q,token)
            deadline=time.monotonic()+600
            while time.monotonic()<deadline:
                job=request(api+"job",token=token)
                if job["state"]!="running":break
                time.sleep(1)
            assert job["state"]=="done",job
            out=folder/"data"/"outputs"/job["id"]/job["files"][0]
            if kind=="image":
                pixels=np.asarray(Image.open(out));assert pixels.shape[:2]==(256,256) and pixels.std()>1
            elif kind=="video":
                with av.open(str(out)) as f:frames=[frame.to_ndarray(format="rgb24") for frame in f.decode(video=0)]
                assert len(frames)==4 and not np.array_equal(frames[0],frames[-1])
            elif kind=="musique":
                with av.open(str(out)) as f:audio=np.concatenate([frame.to_ndarray().ravel() for frame in f.decode(audio=0)])
                assert len(audio)>16000 and audio.std()>0
            else:assert out.read_text(encoding="utf-8").strip()
            print(f"PASS {kind}: {out.name} ({out.stat().st_size} octets)",flush=True)

if __name__=="__main__":run()
