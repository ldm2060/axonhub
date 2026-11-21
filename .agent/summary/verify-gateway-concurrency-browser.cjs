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
  upstreamCalls.push({method:req.method,path:req.url,stream:input.stream,hold:(JSON.stringify(input.input).includes('hold') || JSON.stringify(input.input).includes('stall'))});
  const id = `resp_verify_${++sequence}`;
  const response = { id, object: 'response', created_at: Math.floor(Date.now()/1000), model: 'concurrency-test', status: 'completed', output: [{ id: `msg_${sequence}`, type: 'message', role: 'assistant', status: 'completed', content: [{ type: 'output_text', text: 'ok', annotations: [] }] }], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2 } };
  const finish = () => {
    if (res.destroyed) return;
    if (input.stream) res.end(`event: response.completed\ndata: ${JSON.stringify({ type: 'response.completed', sequence_number: 1, response })}\n\n`);
    else res.end(JSON.stringify(response));
  };
  if(JSON.stringify(input.input).includes('prestart')){pending.set(id,finish);res.on('close',()=>pending.delete(id));return;}
  res.writeHead(200, { 'Content-Type': input.stream ? 'text/event-stream' : 'application/json' });
  if (input.stream) {
    res.write(`event: response.created\ndata: ${JSON.stringify({type: 'response.created', sequence_number: 0, response: { ...response, status: 'in_progress', output: [] }})}\n\n`);
    res.write(`event: response.output_text.delta\ndata: ${JSON.stringify({type:'response.output_text.delta',sequence_number:1,item_id:`msg_${sequence}`,output_index:0,content_index:0,delta:'ok'})}\n\n`);
  }
  if ((JSON.stringify(input.input).includes('hold') || JSON.stringify(input.input).includes('stall'))) {
    const ticker=input.stream && !JSON.stringify(input.input).includes('stall') ? setInterval(()=>res.write(`event: response.output_text.delta\ndata: ${JSON.stringify({type:'response.output_text.delta',sequence_number:sequence++,item_id:'msg_1',output_index:0,content_index:0,delta:'ok'})}\n\n`),400) : null;
    res.on('close',()=>{if(ticker)clearInterval(ticker);});
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
  const secondKey = (await gql('mutation($input:CreateAPIKeyInput!){createAPIKey(input:$input){key}}', {input:{name:`Gateway second key ${Date.now()}`,type:'personal',projectID}})).createAPIKey.key;
  const report = {pageErrors, protocols:[], rejectedRoutes:[], discoveryAtCap:[], webSockets:[]};
  const delay = ms => new Promise(resolve=>setTimeout(resolve,ms));
  const waitPending = async count => {
    for(let i=0;i<200 && pending.size!==count;i++) await delay(20);
    assert.equal(pending.size,count,'upstream active stream count');
  };
  const call = async (path,body,token=apiKey,method='POST') => page.evaluate(async ({path,body,token,method,projectID})=>{
    const response = await fetch(path,{method,headers:{'Content-Type':'application/json',Authorization:`Bearer ${token}`,'X-Project-ID':projectID},...(body!==undefined?{body:JSON.stringify(body)}:{})});
    return {status:response.status,retryAfter:response.headers.get('Retry-After'),body:await response.text()};
  },{path,body,token,method,projectID});
  const fast = () => call('/v1/responses',{model:'concurrency-test',input:'fast',stream:false},secondKey);
  const releasedFast = async () => {
    let result;
    for(let i=0;i<100;i++){result=await fast();if(result.status!==429)return result;await delay(20);}
    return result;
  };
  const protocols = [
    {name:'OpenAI Chat SSE',path:'/v1/chat/completions',body:{model:'concurrency-test',stream:true,messages:[{role:'user',content:'hold'}]}},
    {name:'Anthropic SSE',path:'/v1/messages',body:{model:'concurrency-test',max_tokens:100,stream:true,messages:[{role:'user',content:'hold'}]}},
    {name:'Anthropic alias SSE',path:'/anthropic/v1/messages',body:{model:'concurrency-test',max_tokens:100,stream:true,messages:[{role:'user',content:'hold'}]}},
    {name:'Gemini SSE',path:'/gemini/v1beta/models/concurrency-test:streamGenerateContent?alt=sse',body:{contents:[{role:'user',parts:[{text:'hold'}]}]}},
    {name:'Gemini alias JSON stream',path:'/v1beta/models/concurrency-test:streamGenerateContent',body:{contents:[{role:'user',parts:[{text:'hold'}]}]}},
    {name:'Playground JWT stream',path:'/admin/playground/chat',token:auth.token,body:{model:'concurrency-test',stream:true,messages:[{id:'verify',role:'user',content:'hold',parts:[{type:'text',text:'hold'}]}]}}
  ];
  const start = async (entry,tag) => {
    await page.evaluate(({path,body,token,tag,projectID})=>{
      window.gatewayStreams ??= {};
      const state = {controller:new AbortController(),text:'',done:false};
      window.gatewayStreams[tag]=state;
      (async()=>{
        try {
          const response=await fetch(path,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${token}`,'X-Project-ID':projectID},body:JSON.stringify(body),signal:state.controller.signal});
          state.status=response.status;
          const reader=response.body.getReader(), decoder=new TextDecoder();
          for(;;){const chunk=await reader.read();if(chunk.done)break;state.text+=decoder.decode(chunk.value,{stream:true});}
        } catch(error){state.error=error.name;}
        finally{state.done=true;}
      })();
    },{...entry,token:entry.token||apiKey,tag,projectID});
    await page.waitForFunction(tag=>window.gatewayStreams[tag].text.length>0||window.gatewayStreams[tag].done,tag,{timeout:10000});
    const state=await page.evaluate(tag=>({status:window.gatewayStreams[tag].status,text:window.gatewayStreams[tag].text}),tag);
    assert.equal(state.status,200,`${entry.name}: ${state.text}`);
    await waitPending(1);
  };
  const abort = tag => page.evaluate(tag=>window.gatewayStreams[tag].controller.abort(),tag);
  try {
    await page.goto(base+'/admin/system',{waitUntil:'networkidle'});
    await page.locator('#max-concurrent-requests').waitFor({timeout:20000});
    const skip=page.locator('[data-testid="onboarding-skip-tour"]');
    if(await skip.isVisible())await skip.click();
    await setLimit(1);
    for(const [index,entry] of protocols.entries()){
      const tag=`protocol-${index}`;
      await start(entry,tag);
      const denied=await fast();assert.equal(denied.status,429);assert.equal(denied.retryAfter,'1');
      // All HTTP streams must survive the test server's 1-second non-stream timeout.
      await delay(1200);
      assert.equal(await page.evaluate(tag=>window.gatewayStreams[tag].done,tag),false,entry.name+' total timeout leaked into stream');
      for(const finish of [...pending.values()])finish();
      await page.waitForFunction(tag=>window.gatewayStreams[tag].done,tag,{timeout:10000});
      await waitPending(0);
      assert.equal((await releasedFast()).status,200);
      await start(entry,tag+'-cancel');
      await abort(tag+'-cancel');await waitPending(0);
      assert.equal((await releasedFast()).status,200);
      report.protocols.push({name:entry.name,sharedAcrossKeys:429,survivesNonStreamTimeout:true,completionReleases:true,disconnectReleases:true});
    }
    await start(protocols[0],'at-cap');
    const generationPaths=['/v1/chat/completions','/v1/completions','/v1/responses','/v1/responses/compact','/v1/embeddings','/v1/moderations','/v1/alpha/search','/v1/images/generations','/v1/images/edits','/v1/videos','/v1/audio/speech','/v1/audio/transcriptions','/v1/audio/translations','/v1/messages','/anthropic/v1/messages','/v1/rerank','/jina/v1/embeddings','/jina/v1/rerank','/v1/systemone','/typesafe/v1/systemone','/doubao/v3/contents/generations/tasks','/gemini/v1beta/models/concurrency-test:generateContent','/v1beta/models/concurrency-test:generateContent','/admin/playground/chat'];
    for(const path of generationPaths){
      const r=await call(path,{},path.startsWith('/admin')?auth.token:secondKey);
      assert.equal(r.status,429,path);assert.equal(r.retryAfter,'1');
      const body=JSON.parse(r.body);
      if(path.includes('/gemini/')||path.includes('/v1beta/')){assert.equal(body.error.code,429);assert.equal(body.error.status,'RESOURCE_EXHAUSTED');}
      if(path.endsWith('/messages'))assert.equal(body.type,'error');
      report.rejectedRoutes.push({path,status:r.status});
    }
    for(const path of ['/v1/models','/anthropic/v1/models','/gemini/v1beta/models','/v1beta/models']){
      const r=await call(path,undefined,secondKey,'GET');assert.equal(r.status,200,path);
      report.discoveryAtCap.push({path,status:r.status});
    }
    for(const path of ['/v1/videos/gateway-no-such-task','/doubao/v3/contents/generations/tasks/gateway-no-such-task'])for(const method of ['GET','DELETE']){
      const r=await call(path,undefined,secondKey,method);assert.notEqual(r.status,429,path);
      report.discoveryAtCap.push({path,method,status:r.status});
    }
    const wsEntries=protocols.slice(0,5).map(entry=>({...entry,path:entry.path.split('?')[0]}));
    // Keep all five protocol sockets idle beyond LLMRequestTimeout before use.
    await page.evaluate(async entries=>{
      window.gatewaySockets=[];
      for(const entry of entries){const ws=new WebSocket('ws://127.0.0.1:18991'+entry.path);ws.events=[];ws.onmessage=e=>ws.events.push(e.data);await new Promise((resolve,reject)=>{ws.onopen=resolve;ws.onerror=()=>reject(Error('upgrade failed'));});window.gatewaySockets.push(ws);}
    },wsEntries);
    await delay(1200);
    for(const [index,entry] of wsEntries.entries()){
      await page.evaluate(({index,body})=>window.gatewaySockets[index].send(JSON.stringify(body)),{index,body:entry.body});
      await page.waitForFunction(index=>window.gatewaySockets[index].events.length>0,index,{timeout:5000});
      const data=await page.evaluate(index=>window.gatewaySockets[index].events.join('\n'),index);
      assert.match(data,entry.name.includes('Gemini')?/RESOURCE_EXHAUSTED/:/concurrency_limit_exceeded/);
    }
    await abort('at-cap');await waitPending(0);
    assert.equal((await releasedFast()).status,200);
    for(const [index,entry] of wsEntries.entries()){
      await page.evaluate(({index,body})=>{window.gatewaySockets[index].events=[];window.gatewaySockets[index].send(JSON.stringify(body));},{index,body:entry.body});
      await page.waitForFunction(index=>window.gatewaySockets[index].events.some(e=>e.includes('ok')),index,{timeout:5000});
      await waitPending(1);
      assert.equal((await fast()).status,429);
      await page.evaluate(index=>window.gatewaySockets[index].close(),index);
      await waitPending(0);
      assert.equal((await releasedFast()).status,200);
      report.webSockets.push({path:entry.path,idleBeyondTimeout:true,rejectionAndRetry:true,activeSharedCap:true,disconnectReleases:true});
    }
    report.firstResponseTimeouts=[];
    for(const entry of protocols){
      const before=Date.now();
      const r=await call(entry.path,JSON.parse(JSON.stringify(entry.body).replaceAll('hold','prestart')),entry.token||apiKey);
      assert.notEqual(r.status,200,entry.name+' stalled before first response');
      assert.ok(Date.now()-before>=1800 && Date.now()-before<6000,entry.name+' initial idle timeout');
      await waitPending(0);assert.equal((await releasedFast()).status,200);
      report.firstResponseTimeouts.push({protocol:entry.name,status:r.status,upstreamCanceled:true,budgetReleased:true});
    }
    report.idleTimeouts=[];
    for(const [index,entry] of protocols.entries()){
      const stalled={...entry,body:JSON.parse(JSON.stringify(entry.body).replaceAll('hold','stall'))};
      const tag=`idle-${index}`;
      await start(stalled,tag);
      await page.waitForFunction(tag=>window.gatewayStreams[tag].done,tag,{timeout:6000});
      await waitPending(0);assert.equal((await releasedFast()).status,200);
      const text=await page.evaluate(tag=>window.gatewayStreams[tag].text,tag);
      if(index<3)assert.match(text,/stream idle timeout/);
      if(index===0)assert.match(text,/: keep-alive/);
      if(index===1||index===2)assert.match(text,/"type":"ping"/);
      report.idleTimeouts.push({protocol:entry.name,upstreamCanceled:true,budgetReleased:true});
    }
    // Enabling the limit must count requests admitted while it was disabled.
    await setLimit(0);await start(protocols[0],'disabled-active');await setLimit(1);
    assert.equal((await fast()).status,429);
    await abort('disabled-active');await waitPending(0);assert.equal((await releasedFast()).status,200);
    report.enableWhileActive='passed';
    const timeout=await call('/v1/chat/completions',{model:'concurrency-test',stream:false,messages:[{role:'user',content:'hold'}]});
    assert.notEqual(timeout.status,200);await waitPending(0);assert.equal((await releasedFast()).status,200);
    report.nonStreamingTimeout={status:timeout.status,releases:true};
    await setLimit(6);await page.reload({waitUntil:'networkidle'});
    assert.equal(await page.locator('#max-concurrent-requests').inputValue(),'6');
    if(await skip.isVisible())await skip.click();
    await page.screenshot({path:resolve(__dirname,'gateway-concurrency-settings.png'),fullPage:true});
    assert.deepEqual(pageErrors,[]);report.result='passed';
  } catch(error){
    report.result='failed';report.error=error.message;report.upstreamCalls=upstreamCalls;
    report.streams=await page.evaluate(()=>Object.fromEntries(Object.entries(window.gatewayStreams||{}).map(([key,s])=>[key,{status:s.status,done:s.done,error:s.error,text:s.text}]))).catch(()=>({}));
    console.error(error.stack);process.exitCode=1;
  } finally{
    writeFileSync(resolve(__dirname,'gateway-browser-verification.json'),JSON.stringify(report,null,2));
    console.log(JSON.stringify(report,null,2));await browser.close();
    for(const socket of proxySockets)socket.destroy();socketProxy.close();upstream.closeAllConnections();upstream.close();
  }
})().catch(err=>{console.error(err.message);for(const socket of proxySockets)socket.destroy();socketProxy.close();upstream.closeAllConnections();upstream.close();process.exitCode=1;});
