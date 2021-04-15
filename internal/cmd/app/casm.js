export function connect(cfg) {
    const sock = new WebSocket(cfg.url);
    sock.onmessage = R.compose(cfg.onstep, R.either(reset, step), payload);
}


const reset = R.prop("graph"),
    step = R.prop("step"),
    payload = R.compose(JSON.parse, R.prop("data"));