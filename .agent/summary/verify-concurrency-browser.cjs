const { createRequire } = require('node:module');
const { readFileSync, writeFileSync, existsSync } = require('node:fs');
const { resolve } = require('node:path');
const { createServer, request: httpRequest } = require('node:http');
const { randomBytes } = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = createRequire(resolve(__dirname, '../../frontend/package.json'))('@playwright/test');
const base = 'http://127.0.0.1:8090';
const pending = new Map();
const socketProxy = createServer();
const proxySockets = new Set();
const upstreamCalls = [];
let sequence = 0;
const upstream = createServer(async (req, res) => {
  let body = '';
  for await (const part of req) body += part;
  const input = JSON.parse(body || '{}');
  upstreamCalls.push({method:req.method,path:req.url,stream:input.stream,hold:JSON.stringify(input.input).includes('hold')});
  const id = `resp_verify_${++sequence}`;
  const response = { id, object: 'response', created_at: Math.floor(Date.now()/1000), model: 'concurrency-test', status: 'completed', output: [{ id: `msg_${sequence}`, type: 'message', role: 'assistant', status: 'completed', content: [{ type: 'output_text', text: 'ok', annotations: [] }] }], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2 } };
  const finish = () => {
    if (res.destroyed) return;
    if (input.stream) res.end(`event: response.completed\ndata: ${JSON.stringify({ type: 'response.completed', sequence_number: 1, response })}\n\n`);
    else res.end(JSON.stringify(response));
  };
  res.writeHead(200, { 'Content-Type': input.stream ? 'text/event-stream' : 'application/json' });
  if (input.stream) {
    res.write(`event: response.created\ndata: ${JSON.stringify({type: 'response.created', sequence_number: 0, response: { ...response, status: 'in_progress', output: [] }})}\n\n`);
    res.write(`event: response.output_text.delta\ndata: ${JSON.stringify({type:'response.output_text.delta',sequence_number:1,item_id:`msg_${sequence}`,output_index:0,content_index:0,delta:'ok'})}\n\n`);
  }
  if (JSON.stringify(input.input).includes('hold')) {
    pending.set(id, finish);
    res.on('close', () => pending.delete(id));
  } else finish();
});

async function json(path, body, token) {
  const res = await fetch(base + path, { method: 'POST', headers: { 'Content-Type': 'application/json', ...(token ? {Authorization: `Bearer ${token}`} : {}) }, body: JSON.stringify(body) });
  const data = await res.json();
  assert.equal(res.status, 200, `local API ${path}: ${JSON.stringify(data)}`);
  return data;
}

(async () => {
  await new Promise(resolve => upstream.listen(18990, '127.0.0.1', resolve));
  const credentialPath = resolve(__dirname, 'concurrency-runtime-auth.json');
  let credentials;
  if (existsSync(credentialPath)) credentials = JSON.parse(readFileSync(credentialPath, 'utf8'));
  else {
    credentials = { email: 'concurrency-verify@example.test', password: randomBytes(20).toString('hex') };
    await json('/admin/system/initialize', { ownerEmail: credentials.email, ownerPassword: credentials.password, ownerFirstName: 'Concurrency', ownerLastName: 'Verification', brandName: 'AxonHub Verification', preferLanguage: 'en' });
    writeFileSync(credentialPath, JSON.stringify(credentials));
  }
  const auth = await json('/auth/signin', credentials);
  const gql = async (query, variables = {}) => {
    const result = await json('/admin/graphql', {query, variables}, auth.token);
    assert.equal(result.errors, undefined, JSON.stringify(result.errors));
    return result.data;
  };
  const projects = await gql('query {projects(first:1){edges{node{id}}}}');
  const projectID = projects.projects.edges[0].node.id;
  const channels = await gql('query {channels(first:10){edges{node{id name}}}}');
  let channelID = channels.channels.edges.find(e=>e.node.name==='Concurrency local fixture')?.node.id;
  if (!channelID) {
    const created = await gql('mutation($input:CreateChannelInput!){createChannel(input:$input){id}}', {input:{name:'Concurrency local fixture',type:'openai_responses',baseURL:'http://127.0.0.1:18990/v1',credentials:{apiKey:'local-fixture'},supportedModels:['concurrency-test'],defaultTestModel:'concurrency-test'}});
    channelID = created.createChannel.id;
  }
  await gql('mutation($id:ID!){updateChannelStatus(id:$id,status:enabled){id}}', {id:channelID});
  const keyResult = await gql('mutation($input:CreateAPIKeyInput!){createAPIKey(input:$input){key}}', {input:{name:`Concurrency local verification ${Date.now()}`,type:'personal',projectID}});
  const apiKey = keyResult.createAPIKey.key;
  // Browser WebSocket cannot set Authorization. A loopback proxy supplies only
  // this test API key, while the real gateway still authenticates every upgrade.
  socketProxy.on('upgrade', (req, socket, head) => {
    proxySockets.add(socket);
    const request = httpRequest({hostname:'127.0.0.1',port:8090,path:req.url,method:'GET',headers:{...req.headers,host:'127.0.0.1:8090',authorization:`Bearer ${apiKey}`}});
    request.on('upgrade',(response,target,targetHead)=>{
      socket.write('HTTP/1.1 101 Switching Protocols\r\n'+Object.entries(response.headers).map(([k,v])=>`${k}: ${v}`).join('\r\n')+'\r\n\r\n');
      if(head.length)target.write(head);
      if(targetHead.length)socket.write(targetHead);
      socket.pipe(target);target.pipe(socket);
      socket.on('close',()=>{proxySockets.delete(socket);target.destroy();});
      target.on('error',()=>socket.destroy());
    });
    request.on('response',response=>{socket.end(`HTTP/1.1 ${response.statusCode} Rejected\r\nConnection: close\r\n\r\n`);});
    request.on('error',()=>socket.destroy());
    socket.on('error',()=>request.destroy());
    request.end();
  });
  await new Promise(resolve=>socketProxy.listen(18991,'127.0.0.1',resolve));
  const setLimit = async limit => gql('mutation($input:UpdateConcurrencyLimitSettingsInput!){updateConcurrencyLimitSettings(input:$input)}', {input:{maxConcurrentRequestsPerUser:limit}});
  await setLimit(6);
  const browser = await chromium.launch({headless:true});
  const context = await browser.newContext({viewport:{width:1450,height:1050}});
  await context.addInitScript(({auth}) => {
    localStorage.setItem('axonhub_access_token', auth.token);
    localStorage.setItem('axonhub_user_info', JSON.stringify(auth.user));
    localStorage.setItem('i18nextLng','en');
  }, {auth});
  const page = await context.newPage();
  const pageErrors = [];
  page.on('pageerror', err => pageErrors.push(err.message));
  page.on('console', msg => { if(msg.type()==='error') console.error('browser console:',msg.text()); });
  const report = {limit:6, pageErrors};
  try {
    await page.goto(base + '/admin/system', {waitUntil:'networkidle'});
    await page.locator('#max-concurrent-requests').waitFor({timeout:20000});
    assert.equal(await page.locator('#max-concurrent-requests').inputValue(),'6');
    await page.screenshot({path:resolve(__dirname,'concurrency-settings-browser.png'),fullPage:true});
    report.settingsPage = page.url();
    await page.evaluate(async () => {
      window.verificationSockets = [];
      for (let i=0;i<7;i++) {
        const ws = new WebSocket('ws://127.0.0.1:18991/v1/responses');
        ws.events = [];
        ws.onmessage = e => ws.events.push(JSON.parse(e.data));
        await new Promise((resolve,reject) => { ws.onopen=resolve; ws.onerror=()=>reject(new Error('upgrade failed')); });
        window.verificationSockets.push(ws);
      }
    });
    report.idleWebSockets = 7;
    const post = async input => page.evaluate(async ({apiKey,input}) => {
      const response = await fetch('/v1/responses', {method:'POST', headers:{'Content-Type':'application/json',Authorization:`Bearer ${apiKey}`}, body:JSON.stringify({model:'concurrency-test',input,stream:false})});
      return {status:response.status,retryAfter:response.headers.get('Retry-After'),body:await response.json()};
    }, {apiKey,input});
    const send = async (index,input) => page.evaluate(({index,input}) => window.verificationSockets[index].send(JSON.stringify({type:'response.create',model:'concurrency-test',input})),{index,input});
    const event = async (index,type) => {
      await page.waitForFunction(({index,type})=>window.verificationSockets[index].events.some(e=>e.type===type),{index,type},{timeout:20000});
      return page.evaluate(({index,type})=>window.verificationSockets[index].events.find(e=>e.type===type),{index,type});
    };
    report.postWithSevenIdleSockets = (await post('fast')).status;
    assert.equal(report.postWithSevenIdleSockets,200);
    await send(0,'hold');
    await event(0,'response.created');
    report.postWithOneActiveGeneration = (await post('fast')).status;
    assert.equal(report.postWithOneActiveGeneration,200);
    for (let i=1;i<6;i++) { await send(i,'hold'); await event(i,'response.created'); }
    report.activeGenerationsAtCap = pending.size;
    assert.equal(report.activeGenerationsAtCap,6);
    const denied = await post('fast');
    report.postAtCap = {status:denied.status,retryAfter:denied.retryAfter,error:denied.body.error.code};
    assert.equal(denied.status,429);
    assert.equal(denied.retryAfter,'1');
    await send(6,'hold');
    const failed = await event(6,'response.failed');
    report.webSocketAtCap = {type:failed.type,code:failed.response.error.code};
    assert.equal(failed.response.error.code,'concurrency_limit_exceeded');
    await setLimit(5);
    report.postAfterLoweringCap = (await post('fast')).status;
    assert.equal(report.postAfterLoweringCap,429);
    await setLimit(7);
    report.postAfterRaisingCap = (await post('fast')).status;
    assert.equal(report.postAfterRaisingCap,200);
    await setLimit(6);
    for (const finish of [...pending.values()]) finish();
    for(let i=0;i<6;i++) await event(i,'response.completed');
    await send(6,'fast');
    await event(6,'response.completed');
    report.sameSocketRetryAfterCompletion = 'response.completed';
    await page.evaluate(()=>window.verificationSockets.forEach(ws=>ws.events=[]));
    await setLimit(1);
    await send(0,'hold');
    await event(0,'response.created');
    await page.evaluate(()=>window.verificationSockets[0].close());
    await page.waitForFunction(()=>window.verificationSockets[0].readyState===WebSocket.CLOSED);
    // A real upstream stream is still awaiting a token; disconnect must cancel it.
    for(let i=0;i<50 && pending.size;i++) await new Promise(resolve=>setTimeout(resolve,20));
    assert.equal(pending.size,0);
    report.postAfterDisconnect = (await post('fast')).status;
    assert.equal(report.postAfterDisconnect,200);
    await setLimit(6);
    await page.evaluate(()=>window.verificationSockets.forEach(ws=>ws.close()));
    await page.reload({waitUntil:'networkidle'});
    assert.equal(await page.locator('#max-concurrent-requests').inputValue(),'6');
    assert.deepEqual(pageErrors,[]);
    report.result='passed';
  } catch (error) {
    report.result='failed'; report.error=error.message;
    report.upstreamCalls=upstreamCalls;
    report.socketEvents=await page.evaluate(()=>window.verificationSockets?.map(ws=>ws.events)).catch(()=>[]);
    console.error(error.message);
    await page.screenshot({path:resolve(__dirname,'concurrency-browser-failure.png'),fullPage:true}).catch(()=>{});
    process.exitCode=1;
  } finally {
    writeFileSync(resolve(__dirname,'concurrency-browser-verification.json'),JSON.stringify(report,null,2));
    console.log(JSON.stringify(report,null,2));
    await browser.close();
    for(const socket of proxySockets)socket.destroy();
    socketProxy.close();
    upstream.closeAllConnections(); upstream.close();
  }
})().catch(err=>{ console.error(err.message);for(const socket of proxySockets)socket.destroy();socketProxy.close();upstream.closeAllConnections();upstream.close();process.exitCode=1; });
