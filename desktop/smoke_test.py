"""Smoke test of the actual Linux executable (loopback HTTP), engines absent."""
import json, pathlib, subprocess, tempfile, urllib.request, urllib.error, socket, time, os, re
from html.parser import HTMLParser
base=pathlib.Path(__file__).resolve().parents[1]
class Page(HTMLParser):
    def __init__(self):super().__init__();self.ids=[];self.kinds=[]
    def handle_starttag(self,tag,attrs):
        a=dict(attrs)
        if 'id' in a:self.ids.append(a['id'])
        if tag=='button' and 'data-kind' in a:self.kinds.append(a['data-kind'])
def smoke():
    with tempfile.TemporaryDirectory() as tmp:
        with socket.socket() as sock:sock.bind(('127.0.0.1',0));dead=sock.getsockname()[1]
        endpoint=f'http://127.0.0.1:{dead}'
        p=subprocess.Popen([str(base/'IA-Locale-Studio-Linux'),'-no-browser','-root',tmp,'-port','0','-ollama',endpoint,'-comfy',endpoint],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
        try:
            line=p.stdout.readline();url=re.search(r'http://localhost:\d+/',line).group(0).replace('localhost','127.0.0.1');
            def request(path,body=None,token=None):
                headers={}
                if token:headers['X-Studio-Token']=token
                if body is not None:headers['Content-Type']='application/json'
                req=urllib.request.Request(url+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
                try:
                    with urllib.request.urlopen(req,timeout=30) as r:return r.status,r.read(),dict(r.headers)
                except urllib.error.HTTPError as e:return e.code,e.read(),dict(e.headers)
            status,html,h=request('');assert status==200 and b'IA Locale' in html;assert 'Content-Security-Policy' in h;print('PASS executable Linux : serveur et HTML réels')
            page=Page();page.feed(html.decode());assert len(page.ids)==len(set(page.ids));assert page.kinds==['texte','code','image','video','musique'];code,js,_=request('app.js');assert code==200;refs=set(re.findall(r"\$\('([^']+)'\)",js.decode()));assert refs<=set(page.ids),refs-set(page.ids);assert request('style.css')[0]==200;print(f'PASS {len(page.ids)} identifiants uniques, {len(refs)} références JS, cinq sections, CSS servi')
            status,b,h=request('api/session');assert status==200;token=json.loads(b)['token'];assert 'HttpOnly' in h['Set-Cookie'];assert request('api/status')[0]==403;print('PASS accès API protégé')
            status,b,_=request('api/status',token=token);v=json.loads(b);assert status==200 and v['ollama_error'] and v['comfy_error'] and not v['models'];print('PASS moteurs absents correctement signalés')
            assert request('api/generate',{'kind':'texte','prompt':'test','model':'local:1','context':2048,'tokens':128},token)[0]==202
            for _ in range(50):
                j=json.loads(request('api/job',token=token)[1]);
                if j['state']!='running':break
                time.sleep(.1)
            assert j['state']=='error' and not j['files'];print('PASS aucun faux succès sans moteur')
            assert request('api/backup',{},token)[0]==202
            for _ in range(100):
                j=json.loads(request('api/job',token=token)[1]);
                if j['state']!='running':break
                time.sleep(.05)
            assert j['state']=='done' and (pathlib.Path(tmp)/'backups'/(j['id']+'.zip')).is_file();print('PASS sauvegarde réelle sur disque')
            rss=next((line.strip() for line in pathlib.Path(f'/proc/{p.pid}/status').read_text().splitlines() if line.startswith('VmRSS:')),'inconnu');print('Mesure ponctuelle du serveur Linux sans moteur ni navigateur : '+rss)
        finally:p.terminate();p.wait(timeout=5)
if __name__=='__main__':smoke()
