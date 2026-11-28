"""Local ComfyUI nodes for exercising Studio with trained CPU models."""
import os
from pathlib import Path
import numpy as np
import torch

ROOT = Path(os.environ["STUDIO_TEST_MODELS"])
torch.set_num_threads(2)

class Illustration:
    @classmethod
    def INPUT_TYPES(cls):
        return {"required":{"prompt":("STRING",{}),"frames":("INT",{"default":1,"min":1,"max":6})}}
    RETURN_TYPES=("IMAGE",)
    FUNCTION="generate"
    CATEGORY="Studio tests"

    def generate(self,prompt,frames):
        from diffusers import StableDiffusionPipeline
        pipe=StableDiffusionPipeline.from_pretrained(ROOT/"tiny",local_files_only=True,safety_checker=None)
        generator=torch.Generator().manual_seed(17)
        first=torch.randn(1,4,32,32,generator=generator)
        second=torch.randn(1,4,32,32,generator=generator)
        images=[]
        for frame in range(frames):
            angle=.18*frame
            latent=first*np.cos(angle)+second*np.sin(angle)
            image=pipe(prompt,height=256,width=256,num_inference_steps=4,latents=latent,guidance_scale=5).images[0]
            images.append(torch.tensor(np.asarray(image).copy(),dtype=torch.float32)/255)
        return (torch.stack(images),)

class Melody:
    @classmethod
    def INPUT_TYPES(cls):
        return {"required":{"prompt":("STRING",{})}}
    RETURN_TYPES=("AUDIO",)
    FUNCTION="generate"
    CATEGORY="Studio tests"

    def generate(self,prompt):
        from transformers import AutoProcessor,MusicgenForConditionalGeneration
        model=MusicgenForConditionalGeneration.from_pretrained(ROOT/"music",local_files_only=True).eval()
        processor=AutoProcessor.from_pretrained(ROOT/"music",local_files_only=True)
        inputs=processor(text=[prompt],padding=True,return_tensors="pt")
        torch.manual_seed(17)
        waveform=model.generate(**inputs,max_new_tokens=80,do_sample=True)
        return ({"waveform":waveform,"sample_rate":model.config.audio_encoder.sampling_rate},)

NODE_CLASS_MAPPINGS={"StudioTestIllustration":Illustration,"StudioTestMelody":Melody}
