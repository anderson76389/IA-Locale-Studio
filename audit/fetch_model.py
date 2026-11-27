import concurrent.futures, hashlib, json, pathlib, subprocess, sys
root = pathlib.Path(sys.argv[1]).resolve()
url = 'https://registry.ollama.ai/v2/library/smollm'
manifest = subprocess.check_output(['curl','-fLsS','--max-time','45',url+'/manifests/135m'])
meta=json.loads(manifest)
def blob(layer):
    digest=layer['digest']
    out=root/'blobs'/digest.replace(':','-')
    out.parent.mkdir(parents=True,exist_ok=True)
    subprocess.run(['curl','-fLsS','--retry','2','--max-time','100',url+'/blobs/'+digest,'-o',str(out)],check=True)
    assert out.stat().st_size == layer['size']
    with out.open('rb') as stream:
        assert hashlib.file_digest(stream,'sha256').hexdigest() == digest.split(':')[1]
    print(out.name, layer['size'], flush=True)
with concurrent.futures.ThreadPoolExecutor(max_workers=5) as pool:
    list(pool.map(blob,[meta['config'],*meta['layers']]))
path=root/'manifests/registry.ollama.ai/library/smollm/135m'
path.parent.mkdir(parents=True,exist_ok=True)
path.write_bytes(manifest)
