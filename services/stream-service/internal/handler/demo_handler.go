package handler

import "net/http"

// Demo serves a minimal browser page that connects to the WebRTC signaling
// WebSocket as a viewer and renders every relay event live -- viewer-count
// updates, offer/answer/ICE frames if a broadcaster is connected, join/leave
// events. It's a debugging/demo aid, not part of the product surface.
func (h *Handler) Demo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`
<!doctype html>
<html>
<head>
<title>stream-service — live signaling demo</title>
<meta charset="utf-8">
<style>
  body { font-family: -apple-system, system-ui, sans-serif; background: #0b0f14; color: #e6edf3; margin: 0; padding: 2rem; }
  h1 { font-size: 1.1rem; font-weight: 600; color: #9fb3c8; margin: 0 0 1rem; }
  .row { display: flex; gap: 0.75rem; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; }
  input { background: #11161d; border: 1px solid #30363d; color: #e6edf3; padding: 0.5rem 0.75rem; border-radius: 6px; font-size: 0.85rem; width: 340px; }
  button { background: #238636; border: none; color: white; padding: 0.5rem 1rem; border-radius: 6px; font-size: 0.85rem; cursor: pointer; }
  button:hover { background: #2ea043; }
  #status { font-size: 0.85rem; color: #6b7c8f; margin-bottom: 1rem; }
  #status.live { color: #39d3ac; }
  .count { font-size: 2.5rem; font-weight: 700; color: #39d3ac; }
  .count-label { font-size: 0.8rem; color: #6b7c8f; text-transform: uppercase; letter-spacing: 0.05em; }
  #log { font-family: ui-monospace, monospace; font-size: 0.8rem; background: #11161d; border: 1px solid #30363d; border-radius: 8px; padding: 1rem; height: 400px; overflow-y: auto; }
  .entry { padding: 0.25rem 0; border-bottom: 1px solid #1c2530; }
  .entry .type { color: #39d3ac; font-weight: 600; }
  .entry .time { color: #6b7c8f; margin-right: 0.5rem; }
</style>
</head>
<body>
<h1>stream-service — live WebRTC signaling demo (viewer role)</h1>
<div class="row">
  <input id="streamId" placeholder="stream id" />
  <button onclick="connect()">Connect as viewer</button>
</div>
<div id="status">not connected</div>
<div class="row">
  <div>
    <div class="count" id="count">0</div>
    <div class="count-label">live viewers</div>
  </div>
</div>
<div id="log"></div>
<script>
  const statusEl = document.getElementById('status');
  const countEl = document.getElementById('count');
  const logEl = document.getElementById('log');

  function log(type, data) {
    const div = document.createElement('div');
    div.className = 'entry';
    const time = new Date().toLocaleTimeString();
    div.innerHTML = '<span class="time">' + time + '</span><span class="type">' + type + '</span> ' + JSON.stringify(data);
    logEl.prepend(div);
  }

  function connect() {
    const id = document.getElementById('streamId').value.trim();
    if (!id) return;
    const ws = new WebSocket('ws://' + location.host + '/streams/' + id + '/signal?role=viewer');
    ws.onopen = () => { statusEl.textContent = 'live — connected'; statusEl.className = 'live'; log('connected', {stream_id: id}); };
    ws.onclose = () => { statusEl.textContent = 'disconnected'; statusEl.className = ''; log('disconnected', {}); };
    ws.onerror = () => { statusEl.textContent = 'connection error (is the stream live?)'; };
    ws.onmessage = (evt) => {
      const msg = JSON.parse(evt.data);
      log(msg.type, msg);
      if (msg.type === 'viewer-count') countEl.textContent = msg.payload.count;
    };
  }
</script>
</body>
</html>
`))
}
