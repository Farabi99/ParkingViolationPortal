const API_BASE = 'http://localhost:8085';

const http = require('node:http');

async function test() {
  const data = JSON.stringify({ username: 'officer1', password: 'password' });
  const req = http.request({
    hostname: 'localhost',
    port: 8085,
    path: '/api/auth/login',
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(data)
    }
  }, (res) => {
    console.log('Status:', res.statusCode);
    console.log('Headers:', res.headers);
    let body = '';
    res.on('data', c => body+=c);
    res.on('end', () => console.log('Body:', body));
  });
  req.write(data);
  req.end();
}
test();
