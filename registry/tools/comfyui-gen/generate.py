#!/usr/bin/env python3
"""ComfyUI image generator - CPL tool backend"""
import json, urllib.request, time, sys, os, argparse

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--prompt', default='a cute cat')
    parser.add_argument('--port', default='8194')
    parser.add_argument('--width', type=int, default=1024)
    parser.add_argument('--height', type=int, default=1024)
    parser.add_argument('--steps', type=int, default=20)
    parser.add_argument('--model', default='sd_xl_base_1.0.safetensors')
    parser.add_argument('--output', default='')
    args = parser.parse_args()

    base_url = f'http://127.0.0.1:{args.port}'
    
    # Check ComfyUI is running
    try:
        urllib.request.urlopen(f'{base_url}/system_stats', timeout=5)
    except:
        print(f'Error: ComfyUI not running on port {args.port}. Start it first.')
        sys.exit(1)

    # Simple SDXL txt2img workflow
    workflow = {
        "3": {
            "class_type": "KSampler",
            "inputs": {"seed": int(time.time()), "steps": args.steps, "cfg": 7,
                "sampler_name": "euler", "scheduler": "normal", "denoise": 1.0,
                "model": ["4", 0], "positive": ["6", 0], "negative": ["7", 0],
                "latent_image": ["5", 0]}
        },
        "4": {
            "class_type": "CheckpointLoaderSimple",
            "inputs": {"ckpt_name": args.model}
        },
        "5": {
            "class_type": "EmptyLatentImage",
            "inputs": {"width": args.width, "height": args.height, "batch_size": 1}
        },
        "6": {
            "class_type": "CLIPTextEncode",
            "inputs": {"text": args.prompt, "clip": ["4", 1]}
        },
        "7": {
            "class_type": "CLIPTextEncode",
            "inputs": {"text": "blurry, low quality", "clip": ["4", 1]}
        },
        "8": {
            "class_type": "VAEDecode",
            "inputs": {"samples": ["3", 0], "vae": ["4", 2]}
        },
        "9": {
            "class_type": "SaveImage",
            "inputs": {"filename_prefix": "unicli_gen", "images": ["8", 0]}
        }
    }

    # Submit workflow
    payload = json.dumps({"prompt": workflow}).encode()
    req = urllib.request.Request(f'{base_url}/api/prompt', data=payload,
        headers={'Content-Type': 'application/json'})
    resp = urllib.request.urlopen(req, timeout=30)
    result = json.loads(resp.read())
    prompt_id = result['prompt_id']
    print(f'Job submitted: {prompt_id}', file=sys.stderr)

    # Poll for completion
    for _ in range(120):
        time.sleep(2)
        try:
            req = urllib.request.Request(f'{base_url}/api/history/{prompt_id}')
            resp = urllib.request.urlopen(req, timeout=5)
            history = json.loads(resp.read())
            if prompt_id in history:
                outputs = history[prompt_id].get('outputs', {})
                if outputs:
                    for nid, no in outputs.items():
                        for t, files in no.items():
                            for f in files:
                                fn = f.get('filename', '')
                                sub = f.get('subfolder', '')
                                img_path = os.path.join(
                                    os.path.expanduser('~'), 'Documents', 'comfy', 'ComfyUI', 'output',
                                    sub, fn
                                )
                                # Copy to desktop or output dir
                                out_dir = args.output or os.path.expanduser('~/Desktop')
                                os.makedirs(out_dir, exist_ok=True)
                                out_path = os.path.join(out_dir, fn)
                                if os.path.exists(img_path):
                                    import shutil
                                    shutil.copy2(img_path, out_path)
                                    print(f'✅ Image: {out_path}')
                                    print(f'   Prompt: {args.prompt}')
                                    print(f'   Seed: {workflow["3"]["inputs"]["seed"]}')
                                    return
        except:
            continue
    
    print('Error: timeout waiting for image generation', file=sys.stderr)
    sys.exit(1)

if __name__ == '__main__':
    main()
