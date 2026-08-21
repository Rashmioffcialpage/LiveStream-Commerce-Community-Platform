package handler

import "net/http"

// Demo is a debugging/demo aid, same purpose as stream-service's GET
// /demo: connects to live chat with a real JWT and renders every event
// live, so the chat pipeline can be watched working without a full
// frontend client.
func (h *Handler) Demo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`
<!doctype html>
<html>
<head>
<title>chat-service — live chat demo</title>
<meta charset="utf-8">
<style>
  body { font-family: -apple-system, system-ui, sans-serif; background: #0b0f14; color: #e6edf3; margin: 0; padding: 2rem; }
  h1 { font-size: 1.1rem; font-weight: 600; color: #9fb3c8; margin: 0 0 1rem; }
  .row { display: flex; gap: 0.75rem; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; }
  input { background: #11161d; border: 1px solid #30363d; color: #e6edf3; padding: 0.5rem 0.75rem; border-radius: 6px; font-size: 0.85rem; }
  input#streamId { width: 340px; }
  input#token { width: 460px; }
  input#body { width: 300px; }
  button { background: #238636; border: none; color: white; padding: 0.5rem 1rem; border-radius: 6px; font-size: 0.85rem; cursor: pointer; }
  button:hover { background: #2ea043; }
  button.reaction { background: #30363d; font-size: 1.1rem; padding: 0.4rem 0.6rem; }
  #status { font-size: 0.85rem; color: #6b7c8f; margin-bottom: 1rem; }
  #status.live { color: #39d3ac; }
  #messages { font-family: ui-monospace, monospace; font-size: 0.85rem; background: #11161d; border: 1px solid #30363d; border-radius: 8px; padding: 1rem; height: 420px; overflow-y: auto; display: flex; flex-direction: column-reverse; }
  .msg { padding: 0.35rem 0; border-bottom: 1px solid #1c2530; }
  .msg .name { color: #39d3ac; font-weight: 600; }
  .msg.reaction .name { color: #f0b93a; }
  .msg.error { color: #f85149; }
  .msg.system { color: #6b7c8f; font-style: italic; }
  .msg .time { color: #4b5563; margin-right: 0.5rem; font-size: 0.75rem; }
</style>
</head>
<body>
<h1>chat-service — live chat demo</h1>
<div class="row">
  <input id="streamId" placeholder="stream id" />
  <input id="token" placeholder="JWT access_token" />
  <button onclick="connect()">Connect</button>
</div>
<div id="status">not connected</div>
<div id="messages"></div>
<div class="row" style="margin-top: 1rem;">
  <input id="body" placeholder="say something..." onkeydown="if(event.key==='Enter') sendMessage()" />
  <button onclick="sendMessage()">Send</button>
  <button class="reaction" onclick="sendReaction('👍')">👍</button>
  <button class="reaction" onclick="sendReaction('🔥')">🔥</button>
  <button class="reaction" onclick="sendReaction('❤️')">❤️</button>
  <button class="reaction" onclick="sendReaction('😂')">😂</button>
  <button class="reaction" onclick="sendReaction('🎉')">🎉</button>
</div>
<script>
  const statusEl = document.getElementById('status');
  const messagesEl = document.getElementById('messages');
  let ws = null;

  function addMsg(cls, html) {
    const div = document.createElement('div');
    div.className = 'msg ' + cls;
    const time = new Date().toLocaleTimeString();
    div.innerHTML = '<span class="time">' + time + '</span>' + html;
    messagesEl.prepend(div);
  }

  function connect() {
    const id = document.getElementById('streamId').value.trim();
    const token = document.getElementById('token').value.trim();
    if (!id || !token) return;
    ws = new WebSocket('ws://' + location.host + '/streams/' + id + '/chat?token=' + encodeURIComponent(token));
    ws.onopen = () => { statusEl.textContent = 'live — connected'; statusEl.className = 'live'; };
    ws.onclose = () => { statusEl.textContent = 'disconnected'; statusEl.className = ''; };
    ws.onerror = () => { statusEl.textContent = 'connection error'; };
    ws.onmessage = (evt) => {
      const msg = JSON.parse(evt.data);
      if (msg.type === 'history') {
        (msg.messages || []).forEach(m => addMsg(m.type, '<span class="name">' + m.display_name + ':</span> ' + m.body));
        addMsg('system', '--- history loaded ---');
      } else if (msg.type === 'message' || msg.type === 'reaction') {
        addMsg(msg.type, '<span class="name">' + msg.display_name + ':</span> ' + msg.body);
      } else if (msg.type === 'error') {
        addMsg('error', 'rejected: ' + msg.reason);
      } else if (msg.type === 'message-deleted') {
        addMsg('system', 'a message was deleted by a moderator');
      }
    };
  }

  function sendMessage() {
    const body = document.getElementById('body').value.trim();
    if (!body || !ws) return;
    ws.send(JSON.stringify({type: 'message', body}));
    document.getElementById('body').value = '';
  }

  function sendReaction(emoji) {
    if (!ws) return;
    ws.send(JSON.stringify({type: 'reaction', body: emoji}));
  }

  const params = new URLSearchParams(location.search);
  if (params.get('stream_id')) document.getElementById('streamId').value = params.get('stream_id');
  if (params.get('token')) document.getElementById('token').value = params.get('token');
  if (params.get('autoconnect') === '1' && params.get('stream_id') && params.get('token')) connect();
</script>
</body>
</html>
`))
}
