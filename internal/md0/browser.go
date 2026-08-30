package md0

import (
	"html"
	"strings"
)

const runtimeStatusCSS = `
.md0-status{position:fixed;right:20px;bottom:20px;z-index:50;max-width:min(560px,calc(100vw - 40px));padding:12px 14px;border:1px solid #ebc7c0;border-radius:12px;background:#fff0ed;color:#8f2f2f;box-shadow:0 12px 32px rgba(35,31,32,.12);font:14px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace;white-space:pre-wrap}.md0-status[hidden]{display:none}.md0-input input[aria-invalid="true"]{border-color:#a53838;box-shadow:0 0 0 3px rgba(165,56,56,.12);outline:none}
`

const runtimeStatusJS = `
function md0RuntimeToken(){const meta=document.querySelector('meta[name="md0-runtime-token"]');return meta?meta.content:''}
function md0SetRuntimeError(message,inputName){const status=document.getElementById('md0-status');status.textContent=message;status.hidden=false;document.querySelectorAll('[data-md0-input]').forEach(el=>{if(inputName&&el.name===inputName){el.setAttribute('aria-invalid','true')}else{el.removeAttribute('aria-invalid')}})}
function md0ClearRuntimeError(){const status=document.getElementById('md0-status');status.textContent='';status.hidden=true;document.querySelectorAll('[data-md0-input]').forEach(el=>el.removeAttribute('aria-invalid'))}
md0SendLatest=async function(){const values={};document.querySelectorAll('[data-md0-input]').forEach(el=>{values[el.name]=el.type==='checkbox'?String(el.checked):el.value});let r;try{r=await fetch('/render',{method:'POST',headers:{'content-type':'application/json','x-md0-token':md0RuntimeToken()},body:JSON.stringify(values)})}catch(err){md0SetRuntimeError('md0 runtime unavailable: '+err.message,'');return}let payload;try{payload=await r.json()}catch{md0SetRuntimeError('md0 runtime returned an invalid response','');return}if(!r.ok){md0SetRuntimeError(payload.error||'md0 could not evaluate this document',payload.input||'');return}md0ClearRuntimeError();for(const patch of payload.patches){const node=document.getElementById(patch.dom_id);if(!node){console.warn('md0 patch target missing',patch.node);continue}node.outerHTML=patch.html}}
`

func renderInteractiveRuntimePage(title, fragment, token string) string {
	page := RenderInteractivePage(title, fragment)
	page = strings.Replace(page, `</head>`, `<meta name="md0-runtime-token" content="`+html.EscapeString(token)+`"><style>`+runtimeStatusCSS+`</style></head>`, 1)
	page = strings.Replace(page, `<body>`, `<body><aside id="md0-status" class="md0-status" role="status" aria-live="polite" hidden></aside>`, 1)
	page = strings.Replace(page, `</body>`, `<script>`+runtimeStatusJS+`</script></body>`, 1)
	return page
}
