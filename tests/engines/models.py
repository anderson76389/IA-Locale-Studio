"""Download only the pinned weights used by the real-engine tests."""
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import subprocess

models = [
    ("segmind/tiny-sd","cad0bd7495fa6c4bcca01b19a723dc91627fe84f","tiny",[
        "model_index.json","scheduler/scheduler_config.json","text_encoder/config.json",
        "text_encoder/pytorch_model.bin","tokenizer/merges.txt","tokenizer/special_tokens_map.json",
        "tokenizer/tokenizer_config.json","tokenizer/vocab.json","unet/config.json",
        "unet/diffusion_pytorch_model.bin","vae/config.json","vae/diffusion_pytorch_model.bin"]),
    ("facebook/musicgen-small","4c8334b02c6ec4e8664a91979669a501ec497792","music",[
        "config.json","generation_config.json","model.safetensors","preprocessor_config.json",
        "special_tokens_map.json","spiece.model","tokenizer.json","tokenizer_config.json"]),
    ("bartowski/SmolLM2-135M-Instruct-GGUF","09816acd5d99df7be770d85ea30822623dab342c","text",[
        "SmolLM2-135M-Instruct-Q4_K_M.gguf"]),
]

def fetch(job):
    url,dest=job
    dest.parent.mkdir(exist_ok=True,parents=True)
    temp=dest.with_suffix(dest.suffix+".part")
    subprocess.run(["curl","-fsSL","--retry","3",url,"-o",str(temp)],check=True)
    temp.replace(dest)

if __name__=="__main__":
    jobs=[(f"https://huggingface.co/{repo}/resolve/{ref}/{name}",Path(folder,name))
          for repo,ref,folder,files in models for name in files]
    with ThreadPoolExecutor(4) as pool:list(pool.map(fetch,jobs))
