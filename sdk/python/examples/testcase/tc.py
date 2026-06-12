#!/usr/bin/env python3
"""测试平台 CLI - 智能测试用例平台命令行工具"""
import argparse, json, os, sys, urllib.request, urllib.error

# 配置
CONFIG_DIR = os.path.expanduser("~/.unicli/tc")
os.makedirs(CONFIG_DIR, exist_ok=True)
CONFIG_FILE = os.path.join(CONFIG_DIR, "config.json")

# Handle --action flag (unicli mode) - convert to positional arg
if len(sys.argv) > 1 and '--action' in sys.argv:
    idx = sys.argv.index('--action')
    if idx + 1 < len(sys.argv):
        # Move action value to be the first positional arg
        action_val = sys.argv[idx + 1]
        new_args = [sys.argv[0], action_val]
        for i in range(1, len(sys.argv)):
            if i != idx and i != idx + 1:
                new_args.append(sys.argv[i])
        sys.argv = new_args

def load_config():
    try:
        with open(CONFIG_FILE, 'r') as f:
            return json.load(f)
    except:
        return {"url": "http://localhost:8000", "token": ""}

def save_config(cfg):
    with open(CONFIG_FILE, 'w') as f:
        json.dump(cfg, f, indent=2)

def api_get(path, cfg):
    """调用 API"""
    url = f"{cfg['url']}{path}"
    headers = {"Content-Type": "application/json"}
    if cfg.get('token'):
        headers["Authorization"] = f"Bearer {cfg['token']}"
    
    req = urllib.request.Request(url, headers=headers)
    try:
        resp = urllib.request.urlopen(req, timeout=10)
        return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return {"error": f"HTTP {e.code}: {e.read().decode()[:200]}"}
    except urllib.error.URLError as e:
        return {"error": f"连接失败: {e.reason}. Try: tc config --url http://..."}

def main():
    parser = argparse.ArgumentParser(description="🧪 测试平台 CLI")
    parser.add_argument('--url', help='API 地址')
    parser.add_argument('--token', help='认证 Token')
    
    sub = parser.add_subparsers(dest='action', required=True)
    
    # config
    sub.add_parser('config', help='查看/设置配置')
    
    # dashboard
    sub.add_parser('dashboard', help='仪表盘概览')
    
    # cases
    p = sub.add_parser('cases', help='测试用例列表')
    p.add_argument('--page', type=int, default=1)
    p.add_argument('--search', default='')
    p.add_argument('--tags', default='')
    
    # case detail
    p = sub.add_parser('case', help='用例详情')
    p.add_argument('id', help='用例 ID')
    
    # bugs
    p = sub.add_parser('bugs', help='缺陷列表')
    p.add_argument('--page', type=int, default=1)
    p.add_argument('--status', default='')
    
    # execute
    p = sub.add_parser('execute', help='执行测试')
    p.add_argument('--case-id', required=True, help='用例 ID')
    p.add_argument('--env', default='staging', help='环境')
    
    # generate - AI 生成测试用例
    p = sub.add_parser('generate', help='AI 生成测试用例')
    p.add_argument('--prompt', required=True, help='描述要测试的功能')
    p.add_argument('--count', type=int, default=3, help='生成数量')
    
    # pipelines
    sub.add_parser('pipelines', help='流水线列表')
    
    # quality
    sub.add_parser('quality', help='质量洞察')
    
    # health
    sub.add_parser('health', help='健康检查')
    
    args = parser.parse_args()
    
    # Handle config first (doesn't need API)
    if args.action == 'config':
        cfg = load_config()
        if args.url:
            cfg['url'] = args.url
        if args.token:
            cfg['token'] = args.token
        save_config(cfg)
        print(f"✅ 配置已保存:")
        print(f"   URL:   {cfg['url']}")
        print(f"   Token: {cfg['token'][:20]}..." if cfg['token'] else "   Token: (未设置)")
        return
    
    cfg = load_config()
    
    # Actions that use the API
    if args.action == 'health':
        data = api_get("/api/health", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        print(f"✅ 服务状态: {data.get('status', '?')}")
        print(f"   版本: {data.get('version', '?')}")
        return
    
    elif args.action == 'dashboard':
        data = api_get("/api/dashboard/overview", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        print("📊 测试平台概览")
        print(f"   总用例数:    {data.get('totalCases', data.get('total_cases', '?'))}")
        print(f"   今日新增:    {data.get('todayCases', data.get('today_cases', '?'))}")
        print(f"   缺陷:        {data.get('totalBugs', data.get('total_bugs', '?'))}")
        print(f"   自动化率:    {data.get('autoRate', data.get('auto_rate', '?'))}")
        print(f"   平均质量分:  {data.get('avgQuality', data.get('avg_quality', '?'))}")
        return
    
    elif args.action == 'cases':
        params = f"?page={args.page}&search={args.search}&tags={args.tags}"
        data = api_get(f"/api/cases{params}", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        items = data.get('items', data.get('data', []))
        total = data.get('total', len(items))
        print(f"📋 测试用例 (共 {total} 条):")
        for c in items[:20]:
            title = c.get('title', c.get('name', '?'))
            tags = ','.join(c.get('tags', [])[:3])
            score = c.get('qualityScore', c.get('quality_score', ''))
            print(f"  🧪 [{c.get('id','?')}] {title}")
            if tags: print(f"     标签: {tags}")
            if score: print(f"     质量: {score}")
            print()
        return
    
    elif args.action == 'case':
        data = api_get(f"/api/cases/{args.id}", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        print(f"🧪 用例详情: {data.get('title', '?')}")
        print(f"   ID: {data.get('id', '?')}")
        print(f"   摘要: {data.get('summary', '无')}")
        pre = data.get('preconditions', [])
        if pre:
            print(f"   前置条件:")
            for p in pre:
                print(f"     - {p}")
        steps = data.get('steps', [])
        if steps:
            print(f"   步骤:")
            for i, s in enumerate(steps, 1):
                print(f"     {i}. {s}")
        expected = data.get('expectedResults', data.get('expected_results', []))
        if expected:
            print(f"   预期结果:")
            for e in expected:
                print(f"     ✅ {e}")
        return
    
    elif args.action == 'bugs':
        params = f"?page={args.page}"
        if args.status: params += f"&status={args.status}"
        data = api_get(f"/api/bugs{params}", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        items = data.get('items', data.get('data', []))
        print(f"🐛 缺陷列表:")
        for b in items[:20]:
            print(f"  [{b.get('status','?')}] {b.get('title','?')}")
            print(f"     严重: {b.get('severity','?')} | 创建: {b.get('createdAt',b.get('created_at',''))[:10]}")
            print()
        return
    
    elif args.action == 'execute':
        print(f"▶️ 执行测试用例 {args.case_id} 于 {args.env}...")
        try:
            url = f"{cfg['url']}/api/execution"
            headers = {"Content-Type": "application/json"}
            if cfg.get('token'):
                headers["Authorization"] = f"Bearer {cfg['token']}"
            body = json.dumps({"case_id": args.case_id, "environment": args.env}).encode()
            req = urllib.request.Request(url, data=body, headers=headers)
            resp = urllib.request.urlopen(req, timeout=30)
            result = json.loads(resp.read())
            print(f"✅ 执行完成: {result.get('status', result.get('result', '?'))}")
        except Exception as e:
            print(f"❌ 执行失败: {e}")
        return
    
    elif args.action == 'generate':
        print(f"🤖 AI 正在生成测试用例: {args.prompt}")
        print(f"   数量: {args.count}")
        try:
            url = f"{cfg['url']}/api/ai_generator"
            headers = {"Content-Type": "application/json"}
            if cfg.get('token'):
                headers["Authorization"] = f"Bearer {cfg['token']}"
            body = json.dumps({"prompt": args.prompt, "count": args.count}).encode()
            req = urllib.request.Request(url, data=body, headers=headers)
            resp = urllib.request.urlopen(req, timeout=60)
            result = json.loads(resp.read())
            cases = result.get('cases', result.get('data', [result]))
            print(f"\n✅ 生成了 {len(cases)} 条用例:")
            for c in cases:
                print(f"\n  🧪 {c.get('title', '?')}")
                if c.get('steps'):
                    for i, s in enumerate(c['steps'], 1):
                        print(f"     {i}. {s}")
            print()
        except Exception as e:
            print(f"❌ 生成失败: {e}")
        return
    
    elif args.action == 'pipelines':
        data = api_get("/api/pipelines", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        items = data.get('items', data.get('data', [data]))
        print(f"🔧 流水线:")
        for p in items if isinstance(items, list) else [items]:
            print(f"  [{p.get('status','?')}] {p.get('name','?')}")
        return
    
    elif args.action == 'quality':
        data = api_get("/api/quality_insights", cfg)
        if 'error' in data:
            print(f"❌ {data['error']}")
            return
        print("📈 质量洞察:")
        for k, v in (data.items() if isinstance(data, dict) else {}):
            print(f"   {k}: {v}")
        return

if __name__ == '__main__':
    main()
