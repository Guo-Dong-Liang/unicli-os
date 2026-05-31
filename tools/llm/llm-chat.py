#!/usr/bin/env python3
"""LLM chat tool - calls local llama-server API (OpenAI compatible)"""
import json, urllib.request, sys, os, argparse, time

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--prompt', default='你好', help='输入提示词')
    parser.add_argument('--system', default='你是一个乐于助人的助手。', help='系统提示词')
    parser.add_argument('--port', default='8080', help='llama-server 端口')
    parser.add_argument('--temp', type=float, default=0.7, help='温度')
    parser.add_argument('--max-tokens', type=int, default=1024, help='最大生成长度')
    parser.add_argument('--stream', action='store_true', help='流式输出')
    args = parser.parse_args()

    base_url = f'http://127.0.0.1:{args.port}'

    # Check if server is running
    try:
        urllib.request.urlopen(f'{base_url}/health', timeout=5)
    except:
        print(f'Error: llama-server not running on port {args.port}.')
        print(f'  Start it with: double-click start-llama.bat')
        sys.exit(1)

    payload = json.dumps({
        "model": "local-model",
        "messages": [
            {"role": "system", "content": args.system},
            {"role": "user", "content": args.prompt}
        ],
        "temperature": args.temp,
        "max_tokens": args.max_tokens,
        "stream": args.stream
    }).encode()

    req = urllib.request.Request(
        f'{base_url}/v1/chat/completions',
        data=payload,
        headers={'Content-Type': 'application/json'}
    )

    try:
        resp = urllib.request.urlopen(req, timeout=60)
    except urllib.error.HTTPError as e:
        print(f'Error: API returned {e.code}')
        if e.code == 404:
            print('  The llama-server version may not support /v1/chat/completions.')
            print('  Try: curl http://127.0.0.1:8080/v1/chat/completions -d \'{"messages":[{"role":"user","content":"hi"}]}\'')
            print('  Or check if the model is loaded correctly.')
        sys.exit(1)
    except urllib.error.URLError as e:
        print(f'Error: {e.reason}')
        sys.exit(1)

    result = json.loads(resp.read())
    content = result.get('choices', [{}])[0].get('message', {}).get('content', '')

    # Print the response with some formatting
    print(f'\n🤖 {content.strip()}')
    print(f'\n---')
    print(f'Prompt: {args.prompt[:40]}{"..." if len(args.prompt) > 40 else ""}')
    usage = result.get('usage', {})
    if usage:
        print(f'Tokens: {usage.get("total_tokens", "?")} total')

if __name__ == '__main__':
    main()
