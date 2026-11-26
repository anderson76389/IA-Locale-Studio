"""Integration test of compiled Linux server + Chromium, with simulated engines."""
import http.server, threading, json, subprocess, tempfile, time, urllib.request, pathlib, sys
from playwright.sync_api import sync_playwright
class Engine(http.server.BaseHTTPRequestHandler):
    def log_message(self,*args): pass
    def send(self,data):
        self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers();self.wfile.write(json.dumps(data).encode())
    def do_GET(self):
        if self.path=='/api/tags':self.send({'models':[{'name':'local:1','size':4000000000,'details':{'format':'gguf'}}]})
        elif self.path=='/system_stats':self.send({'system':{},'devices':[]})
        elif self.path.startswith('/history/'): self.send({'test':{'status':{'completed':True,'status_str':'success'},'outputs':{'1':{'images':[{'filename':'out.png','type':'output','subfolder':''}]}}}})
        elif self.path.startswith('/view?'): self.send_response(200);self.end_headers();self.wfile.write(b'test image bytes')
        else:self.send_error(404)
    def do_POST(self):
        data=json.loads(self.rfile.read(int(self.headers.get('Content-Length',0))))
        if self.path=='/api/chat':
            self.send_response(200);self.end_headers();self.wfile.write(b'{"message":{"content":"Bonjour test"},"done":true,"eval_count":2,"eval_duration":1000000000}\n')
        elif self.path=='/prompt':self.send({'prompt_id':'test'})
        else:self.send_error(404)
def wait_url(url):
    for _ in range(100):
        try:
            with urllib.request.urlopen(url,timeout=.2):return
        except Exception:time.sleep(.05)
    raise RuntimeError('server not started')
def run():
    engine=http.server.ThreadingHTTPServer(('127.0.0.1',0),Engine);threading.Thread(target=engine.serve_forever,daemon=True).start()
    base='http://127.0.0.1:'+str(engine.server_port);binary=pathlib.Path(__file__).resolve().parents[1]/'IA-Locale-Studio-Linux'
    with tempfile.TemporaryDirectory() as tmp:
        server=subprocess.Popen([str(binary),'-no-browser','-root',tmp,'-port','0','-ollama',base,'-comfy',base],stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
        try:
            line=server.stdout.readline();url=line.split('http://localhost:')[1].strip();url='http://127.0.0.1:'+url;wait_url(url)
            with sync_playwright() as p:
                browser=p.chromium.launch(executable_path='/usr/bin/chromium',args=['--no-sandbox']);page=browser.new_page(accept_downloads=True);errors=[];page.on('pageerror',lambda e:errors.append(str(e)));page.goto(url);page.locator('#models option[value="local:1"]').wait_for(state='attached')
                for w,h in [(1440,900),(768,1024),(390,844),(320,720)]:
                    page.set_viewport_size({'width':w,'height':h})
                    for k in ['texte','code','image','video','musique']:
                        page.locator('button[data-kind='+k+']').click();assert page.locator('button.selected').get_attribute('data-kind')==k
                        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth+1'),(w,h,k)
                        assert page.locator('#media-settings').is_visible()==(k in ['image','video','musique'])
                    print(f'PASS {w}×{h}: cinq sections, sélection, formulaires, aucun débordement')
                page.set_viewport_size({'width':1200,'height':900});page.locator('button[data-kind=texte]').click();page.locator('#prompt').fill('Test');page.locator('#generate').click();page.wait_for_function("document.querySelector('#state').textContent==='Terminé'",timeout=10000);assert 'Bonjour test' in page.locator('#result').inner_text();print('PASS chat simulé dans l’interface')
                with page.expect_download() as d:page.locator('#files button').first.click()
                assert d.value.suggested_filename=='reponse.txt';assert pathlib.Path(d.value.path()).read_text()=='Bonjour test';print('PASS téléchargement natif sans blob mémoire')
                page.locator('button[data-kind=image]').click();page.locator('#workflow').set_input_files({'name':'workflow.json','mimeType':'application/json','buffer':b'{"1":{"class_type":"CLIPTextEncode","inputs":{"text":"{{PROMPT}}"}}}'});page.locator('#trusted').check();page.locator('#prompt').fill('image test');page.locator('#generate').click();page.wait_for_function("document.querySelector('#state').textContent==='Terminé' && document.querySelector('#files').textContent.includes('creation-001.png')",timeout=10000);print('PASS workflow média simulé via l’interface')
                page.locator('#settings').click();page.on('dialog',lambda d:d.accept());page.locator('#backup').click();page.wait_for_function("document.querySelector('#maintenance').textContent.includes('Sauvegarde vérifiée')",timeout=10000);page.wait_for_function("document.querySelector('#backup-list').options.length>1",timeout=10000);page.locator('#backup-list').select_option(index=1);page.locator('#restore').click();page.wait_for_function("document.querySelector('#maintenance').textContent.includes('Restauration vérifiée')",timeout=10000);print('PASS sauvegarde/restauration via interface');assert not errors,errors;print('PASS aucune erreur JavaScript non interceptée');browser.close()
        finally:server.terminate();server.wait(timeout=5);engine.shutdown()
if __name__=='__main__':run()
