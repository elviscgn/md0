package md0

import (
	"html"
	"strings"
)

const runtimeStatusCSS = `
.md0-tools{position:fixed;right:20px;top:20px;z-index:40;display:flex;gap:8px;padding:8px;border:1px solid #e8dcd0;border-radius:12px;background:rgba(255,252,248,.96);box-shadow:0 8px 24px rgba(35,31,32,.08)}.md0-tools button{padding:7px 10px;border:1px solid #d7cabd;border-radius:8px;background:white;color:#231f20;font:13px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace;cursor:pointer}.md0-tools button:hover{border-color:#c25a2b}.md0-tools button:focus-visible{outline:3px solid rgba(154,70,32,.2);outline-offset:2px}.md0-status{position:fixed;right:20px;bottom:20px;z-index:50;max-width:min(560px,calc(100vw - 40px));padding:12px 14px;border:1px solid #ebc7c0;border-radius:12px;background:#fff0ed;color:#8f2f2f;box-shadow:0 12px 32px rgba(35,31,32,.12);font:14px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace;white-space:pre-wrap}.md0-status[hidden]{display:none}.md0-input input[aria-invalid="true"]{border-color:#a53838;box-shadow:0 0 0 3px rgba(165,56,56,.12);outline:none}@media(max-width:640px){.md0-tools{position:static;margin:12px;justify-content:center}}
`

const runtimeStatusJS = `
function md0RuntimeToken(){const meta=document.querySelector('meta[name="md0-runtime-token"]');return meta?meta.content:''}
function md0InputValues(){const values={};document.querySelectorAll('[data-md0-input]').forEach(el=>{values[el.name]=el.type==='checkbox'?String(el.checked):el.value});return values}
function md0SetRuntimeError(message,inputName){const status=document.getElementById('md0-status');status.textContent=message;status.hidden=false;document.querySelectorAll('[data-md0-input]').forEach(el=>{if(inputName&&el.name===inputName){el.setAttribute('aria-invalid','true')}else{el.removeAttribute('aria-invalid')}})}
function md0ClearRuntimeError(){const status=document.getElementById('md0-status');status.textContent='';status.hidden=true;document.querySelectorAll('[data-md0-input]').forEach(el=>el.removeAttribute('aria-invalid'))}
async function md0Download(endpoint,filename){let r;try{r=await fetch(endpoint,{method:'POST',headers:{'x-md0-token':md0RuntimeToken()}})}catch(err){md0SetRuntimeError('md0 runtime unavailable: '+err.message,'');return}if(!r.ok){let message='md0 could not export this document';try{const payload=await r.json();message=payload.error||message}catch{}md0SetRuntimeError(message,'');return}const blob=await r.blob();const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download=filename;document.body.appendChild(link);link.click();link.remove();URL.revokeObjectURL(url);md0ClearRuntimeError()}
document.getElementById('md0-export-snapshot').addEventListener('click',()=>md0Download('/snapshot','document.snapshot.json'));
document.getElementById('md0-export-markdown').addEventListener('click',()=>md0Download('/markdown','document.md'));
md0SendLatest=async function(){const values=md0InputValues();let r;try{r=await fetch('/render',{method:'POST',headers:{'content-type':'application/json','x-md0-token':md0RuntimeToken()},body:JSON.stringify(values)})}catch(err){md0SetRuntimeError('md0 runtime unavailable: '+err.message,'');return}let payload;try{payload=await r.json()}catch{md0SetRuntimeError('md0 runtime returned an invalid response','');return}if(!r.ok){md0SetRuntimeError(payload.error||'md0 could not evaluate this document',payload.input||'');return}md0ClearRuntimeError();for(const patch of payload.patches){const node=document.getElementById(patch.dom_id);if(!node){console.warn('md0 patch target missing',patch.node);continue}node.outerHTML=patch.html}}
`

func renderInteractiveRuntimePage(title, fragment, token string) string {
	page := RenderInteractivePage(title, fragment)
	page = strings.Replace(page, `</head>`, `<meta name="md0-runtime-token" content="`+html.EscapeString(token)+`"><style>`+runtimeStatusCSS+`</style></head>`, 1)
	page = strings.Replace(page, `<body>`, `<body><nav class="md0-tools" aria-label="Document exports"><button id="md0-export-snapshot" type="button">Export snapshot</button><button id="md0-export-markdown" type="button">Save values to Markdown</button></nav><aside id="md0-status" class="md0-status" role="status" aria-live="polite" hidden></aside>`, 1)
	page = strings.Replace(page, `</body>`, `<script>`+runtimeStatusJS+`</script></body>`, 1)
	return page
}
