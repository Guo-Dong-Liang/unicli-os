#!/usr/bin/env python3
"""微信 CLI - 通过命令行发送和查看消息"""
import json, os, sys, argparse, time

DATA_DIR = os.path.expanduser("~/.unicli/wechat")
os.makedirs(DATA_DIR, exist_ok=True)
CONTACTS_FILE = os.path.join(DATA_DIR, "contacts.json")
MESSAGES_FILE = os.path.join(DATA_DIR, "messages.json")

def load_json(path, default):
    try:
        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except:
        return default

def save_json(path, data):
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)

contacts = load_json(CONTACTS_FILE, [
    {"name": "果果", "emoji": "👦", "note": "儿子"},
    {"name": "爸爸", "emoji": "👨", "note": "我"},
    {"name": "妈妈", "emoji": "👩", "note": "老婆"},
    {"name": "老师", "emoji": "👩‍🏫", "note": "班主任"},
])
save_json(CONTACTS_FILE, contacts)

messages = load_json(MESSAGES_FILE, [
    {"id": 1, "from": "果果", "to": "我", "text": "爸爸，我今天考了100分！", "time": "2026-05-31 10:30"},
    {"id": 2, "from": "我", "to": "果果", "text": "太棒了！晚上带你去吃好吃的 🎉", "time": "2026-05-31 10:31"},
])

def cmd_send(to, msg):
    if not to or not msg:
        return "请指定 --to 和 --msg"
    names = [c['name'] for c in contacts]
    if to not in names:
        return f"联系人 '{to}' 不存在。可用: {', '.join(names)}"
    messages.append({"id": len(messages)+1, "from": "我", "to": to, "text": msg, "time": time.strftime("%Y-%m-%d %H:%M")})
    save_json(MESSAGES_FILE, messages)
    return f"✅ 已发送 → {to}: {msg}"

def cmd_list():
    lines = ["📱 微信联系人\n"]
    for c in contacts:
        lines.append(f"  {c.get('emoji','👤')} {c['name']}{' ('+c['note']+')' if c.get('note') else ''}")
    lines.append(f"\n共 {len(contacts)} 个联系人")
    return "\n".join(lines)

def cmd_history(with_name):
    if not with_name:
        return "请指定 --with 联系人名称"
    conv = [m for m in messages if (m['from']==with_name and m['to']=='我') or (m['from']=='我' and m['to']==with_name)]
    if not conv:
        return f"与 '{with_name}' 暂无聊天记录"
    lines = [f"💬 与 {with_name} 的聊天记录\n"]
    for m in conv:
        sender = "我" if m['from']=='我' else m['from']
        lines.append(f"  [{m['time']}] {sender}: {m['text']}")
    lines.append(f"\n共 {len(conv)} 条消息")
    return "\n".join(lines)

def cmd_stats():
    total = len(messages)
    sent = len([m for m in messages if m['from']=='我'])
    per = {}
    for m in messages:
        other = m['to'] if m['from']=='我' else m['from']
        per[other] = per.get(other,0)+1
    lines = [f"📊 微信统计\n总消息: {total}  已发: {sent}  已收: {total-sent}\n"]
    for name, c in sorted(per.items(), key=lambda x:-x[1]):
        lines.append(f"  {name}: {c} 条")
    return "\n".join(lines)

def main():
    p = argparse.ArgumentParser(description="微信命令行工具")
    p.add_argument('--action', required=True, help='send/list/history/stats')
    p.add_argument('--to', default='', help='接收人')
    p.add_argument('--msg', default='', help='消息内容')
    p.add_argument('--with', dest='chat_with', default='', help='查看聊天记录')
    args = p.parse_args()

    if args.action == 'send': print(cmd_send(args.to, args.msg))
    elif args.action == 'list': print(cmd_list())
    elif args.action == 'history': print(cmd_history(args.chat_with))
    elif args.action == 'stats': print(cmd_stats())
    else: print(f"用法:\n  --action send --to 名称 --msg 内容\n  --action list\n  --action history --with 名称\n  --action stats")

if __name__ == '__main__':
    main()
