package md0

// The authoring surface deliberately stays browser-native. The textarea owns
// input, selection, undo, IME, and accessibility; the layers beneath it add
// md0 syntax color, line numbers, and the active-line treatment.
const editorCSS = `
body.md0-editing{padding-left:min(50vw,780px)}
.md0-editor-pane{--editor-pad-y:18px;--editor-pad-x:24px;--editor-gutter:58px;--editor-line:22px;--editor-coral:#c9573f;--editor-gold:#9b6b1c;--editor-green:#247447;position:fixed;inset:0 auto 0 0;z-index:45;display:flex;flex-direction:column;width:min(50vw,780px);border-right:1px solid var(--line);background:var(--field);color:var(--ink);font-family:var(--md0-font-sans)}.md0-editor-pane[hidden]{display:none}
:root[data-md0-theme="dark"] .md0-editor-pane{--editor-coral:#ff856b;--editor-gold:#dfb96e;--editor-green:#82c99a}
@media(prefers-color-scheme:dark){:root:not([data-md0-theme="light"]):not([data-md0-theme="dark"]) .md0-editor-pane{--editor-coral:#ff856b;--editor-gold:#dfb96e;--editor-green:#82c99a}}
.md0-editor-head{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:55px;padding:10px 13px 10px 16px;border-bottom:1px solid var(--line);background:var(--paper)}
.md0-editor-title{min-width:0}.md0-editor-title strong{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px}.md0-editor-title span{display:block;margin-top:2px;color:var(--muted);font-size:10px}
.md0-editor-actions{display:flex;align-items:center;gap:8px}.md0-editor-save{border:1px solid color-mix(in srgb,var(--editor-coral) 55%,var(--line));border-radius:8px;padding:7px 11px;background:color-mix(in srgb,var(--editor-coral) 10%,var(--paper));color:var(--ink);font:700 11px/1 var(--md0-font-sans);cursor:pointer}.md0-editor-save:hover{background:color-mix(in srgb,var(--editor-coral) 17%,var(--paper))}.md0-editor-save:focus-visible{outline:2px solid var(--editor-coral);outline-offset:2px}
.md0-editor-state{min-width:48px;color:var(--muted);font:650 10px/1 var(--md0-font-sans);text-align:right}.md0-editor-state.error{color:var(--red)}.md0-editor-state.ok{color:var(--editor-green)}.md0-editor-state.dirty{color:var(--editor-gold)}
.md0-editor-code{position:relative;flex:1;min-height:0;overflow:hidden;background:var(--field);isolation:isolate}
.md0-editor-current-line{position:absolute;z-index:0;left:var(--editor-gutter);right:0;height:var(--editor-line);background:color-mix(in srgb,var(--editor-coral) 6%,transparent);border-left:2px solid color-mix(in srgb,var(--editor-coral) 45%,transparent);pointer-events:none}
.md0-editor-gutter{position:absolute;z-index:4;inset:0 auto 0 0;width:var(--editor-gutter);overflow:hidden;border-right:1px solid color-mix(in srgb,var(--line) 80%,transparent);background:color-mix(in srgb,var(--paper) 52%,var(--field));color:var(--faint);font:11px/var(--editor-line) var(--md0-font-mono);text-align:right;pointer-events:none;user-select:none}.md0-editor-gutter-lines{padding:var(--editor-pad-y) 12px 60px 6px}.md0-editor-gutter span{display:block;height:var(--editor-line)}.md0-editor-gutter span.active{color:var(--editor-coral);font-weight:700}
.md0-editor-highlight,.md0-editor-source{position:absolute;inset:0;margin:0;border:0;border-radius:0;padding:var(--editor-pad-y) var(--editor-pad-x) 60px calc(var(--editor-gutter) + 14px);font:13px/var(--editor-line) var(--md0-font-mono);font-variant-ligatures:none;letter-spacing:0;tab-size:2;white-space:pre;word-spacing:0}
.md0-editor-highlight{z-index:1;overflow:hidden;background:transparent;color:var(--ink);pointer-events:none}.md0-editor-highlight code{display:block;min-width:max-content;font:inherit;white-space:inherit}.md0-editor-highlight .tok-directive,.md0-editor-highlight .tok-interpolation{color:var(--editor-coral);font-weight:700}.md0-editor-highlight .tok-version,.md0-editor-highlight .tok-key,.md0-editor-highlight .tok-number{color:var(--editor-gold)}.md0-editor-highlight .tok-string,.md0-editor-highlight .tok-type,.md0-editor-highlight .tok-builtin,.md0-editor-highlight .tok-fence{color:var(--editor-green)}.md0-editor-highlight .tok-heading{color:var(--ink);font-weight:750}.md0-editor-highlight .tok-mark,.md0-editor-highlight .tok-operator,.md0-editor-highlight .tok-comment{color:var(--muted)}.md0-editor-highlight .tok-code{color:var(--editor-green)}.md0-editor-highlight .tok-boolean{color:var(--editor-coral)}.md0-editor-highlight .tok-symbol{color:var(--ink)}
.md0-editor-source{z-index:3;box-sizing:border-box;display:block;width:100%;height:100%;resize:none;outline:0;overflow:auto;background:transparent;color:transparent;-webkit-text-fill-color:transparent;caret-color:var(--ink);scrollbar-color:var(--field-border) transparent;scrollbar-width:thin}.md0-editor-source::selection{background:color-mix(in srgb,var(--editor-coral) 25%,transparent);-webkit-text-fill-color:transparent}.md0-editor-source:focus-visible{box-shadow:inset 2px 0 0 color-mix(in srgb,var(--editor-coral) 72%,transparent)}
.md0-editor-completions{position:absolute;z-index:8;width:min(370px,calc(100% - 76px));max-height:290px;overflow:auto;padding:5px;border:1px solid color-mix(in srgb,var(--line) 88%,var(--editor-coral));border-radius:11px;background:color-mix(in srgb,var(--paper) 98%,transparent);box-shadow:0 18px 54px rgba(0,0,0,.22);backdrop-filter:blur(18px);font-family:var(--md0-font-sans)}.md0-editor-completions[hidden]{display:none}.md0-editor-completion{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:9px;align-items:center;width:100%;border:0;border-radius:7px;padding:7px 8px;background:transparent;color:var(--ink);text-align:left;cursor:pointer}.md0-editor-completion:hover,.md0-editor-completion.active{background:color-mix(in srgb,var(--editor-coral) 11%,var(--surface))}.md0-editor-completion code{color:var(--ink);font:700 12px/1.2 var(--md0-font-mono)}.md0-editor-completion small{overflow:hidden;color:var(--muted);font:10px/1.25 var(--md0-font-sans);text-overflow:ellipsis;white-space:nowrap}.md0-editor-kind{min-width:38px;color:var(--editor-green);font:700 9px/1 var(--md0-font-sans);text-transform:uppercase;letter-spacing:.045em}.md0-editor-completion.active .md0-editor-kind{color:var(--editor-coral)}
.md0-editor-measure{position:absolute;visibility:hidden;white-space:pre;font:13px/var(--editor-line) var(--md0-font-mono);font-variant-ligatures:none;letter-spacing:0;tab-size:2}
.md0-editor-foot{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:28px;padding:5px 13px 5px 15px;border-top:1px solid var(--line);background:var(--paper);color:var(--muted);font:10px/1.2 var(--md0-font-sans)}.md0-editor-foot kbd{border:0;border-radius:4px;padding:2px 4px;background:var(--surface);color:var(--ink);font:9px/1 var(--md0-font-mono)}.md0-editor-position{font-family:var(--md0-font-mono);font-variant-numeric:tabular-nums;white-space:nowrap}.md0-editor-language{color:var(--editor-green);font-weight:700}
.md0-editor-diagnostic{max-height:150px;overflow:auto;padding:10px 14px;border-top:1px solid var(--line);background:var(--paper);color:var(--red);font:11px/1.45 var(--md0-font-mono);white-space:pre-wrap}.md0-editor-diagnostic[hidden]{display:none}
body.md0-editing #md0-document{max-width:var(--page-width);margin-inline:auto}
@media(max-width:900px){body.md0-editing{padding-left:0}.md0-editor-pane{position:relative;width:100%;height:52vh;border-right:0;border-bottom:1px solid var(--line)}.md0-editor-code{min-height:280px}}
`

const editorJS = `
const md0EditorToken=document.querySelector('meta[name="md0-editor-token"]')?.content||'';
const md0EditorID=document.querySelector('meta[name="md0-editor-id"]')?.content||'document';
const md0Editor=document.getElementById('md0-editor-source');
const md0EditorState=document.getElementById('md0-editor-state');
const md0EditorDiagnostic=document.getElementById('md0-editor-diagnostic');
const md0EditorSave=document.getElementById('md0-editor-save');
const md0EditorPane=document.getElementById('md0-editor-pane');
const md0EditorToggle=document.getElementById('md0-edit-source');
const md0EditorHighlight=document.getElementById('md0-editor-highlight-code');
const md0EditorGutter=document.getElementById('md0-editor-gutter-lines');
const md0EditorCurrentLine=document.getElementById('md0-editor-current-line');
const md0EditorCompletions=document.getElementById('md0-editor-completions');
const md0EditorPosition=document.getElementById('md0-editor-position');
const md0EditorMeasure=document.getElementById('md0-editor-measure');
const md0EditorOverridesKey='md0:editor-overrides:'+md0EditorID;
const md0EditorSelectionKey='md0:editor-selection:'+md0EditorID;
const md0ExpressionBuiltins=new Set(['ceil','floor','round','abs','sqrt','min','max','len','sum','avg','get','columns','rows','column']);
const md0PlotBuiltins=new Set(['sin','cos','tan','asin','acos','atan','sqrt','abs','exp','log','ln','log10','floor','ceil','round','pow','min','max']);
const md0InputTypes=new Set(['number','integer','percent','currency','boolean','bool','string','text','duration','json','csv']);
const md0Fence=String.fromCharCode(96).repeat(3);
const md0DirectiveCompletions=[
 {label:'@input',kind:'input',detail:'typed reactive value',insert:'@input name number = 0',select:'name'},
 {label:'@calc',kind:'value',detail:'derived expression',insert:'@calc name = expression',select:'name'},
 {label:'@show',kind:'display',detail:'render one value',insert:'@show value',select:'value'},
 {label:'@when',kind:'block',detail:'conditional Markdown',insert:'@when condition\n\n@end',select:'condition'},
 {label:'@assert',kind:'block',detail:'validated condition',insert:'@assert condition\nExplain the requirement.\n@end',select:'condition'},
 {label:'@data',kind:'data',detail:'host-bound attachment',insert:'@data name json',select:'name'},
 {label:'@table',kind:'block',detail:'reactive table',insert:'@table name\ncolumns = ["Column"]\nrows = [["Value"]]\n@end',select:'name'},
 {label:'@chart',kind:'block',detail:'native bar chart',insert:'@chart name\ntype = bar\nlabels = ["Label"]\nvalues = [value]\n@end',select:'name'},
 {label:'@end',kind:'block',detail:'close a block',insert:'@end'},
 {label:'md0: 0.1',kind:'version',detail:'language declaration',insert:'md0: 0.1'}
];
const md0TableCompletions=[
 {label:'columns',kind:'field',detail:'column headings',insert:'columns = ["Column"]'},
 {label:'rows',kind:'field',detail:'row expressions',insert:'rows = [["Value"]]'},
 {label:'@end',kind:'block',detail:'close table',insert:'@end'}
];
const md0ChartCompletions=[
 {label:'type',kind:'field',detail:'bar in md0/PURE 0.1',insert:'type = bar'},
 {label:'labels',kind:'field',detail:'bar labels',insert:'labels = ["Label"]'},
 {label:'values',kind:'field',detail:'numeric expressions',insert:'values = [value]',select:'value'},
 {label:'@end',kind:'block',detail:'close chart',insert:'@end'}
];
const md0PlotCompletions=[
 {label:'title',kind:'plot',detail:'visible plot title',insert:'title = Plot title',select:'Plot title'},
 {label:'f(x)',kind:'curve',detail:'named curve',insert:'f(x) = sin(x)',select:'sin(x)'},
 {label:'g(x)',kind:'curve',detail:'additional named curve',insert:'g(x) = cos(x)',select:'cos(x)'},
 {label:'y',kind:'plot',detail:'legacy first curve',insert:'y = sin(x)',select:'sin(x)'},
 {label:'y2',kind:'plot',detail:'legacy second curve',insert:'y2 = cos(x)',select:'cos(x)'},
 {label:'label',kind:'plot',detail:'first curve label',insert:'label = Series'},
 {label:'x',kind:'plot',detail:'horizontal domain',insert:'x = [-10, 10]'},
 {label:'samples',kind:'plot',detail:'32 to 1024',insert:'samples = 320'}
];
const md0InputTypeCompletions=['number','integer','percent','currency','boolean','string','duration'].map(label=>({label,kind:'type',detail:'@input value type',insert:label}));
const md0DataFormatCompletions=['json','csv'].map(label=>({label,kind:'format',detail:'host-bound data format',insert:label}));
let md0EditorOverrides=new Set();
try{const stored=JSON.parse(sessionStorage.getItem(md0EditorOverridesKey)||'[]');if(Array.isArray(stored))md0EditorOverrides=new Set(stored.filter(value=>typeof value==='string'))}catch{}
let md0EditorTimer;
let md0EditorBusy=false;
let md0EditorQueued=false;
let md0EditorDirty=false;
let md0CompletionItems=[];
let md0CompletionIndex=0;
let md0CompletionRange={start:0,end:0};

function md0Escape(value){return String(value).replace(/[&<>]/g,char=>char==='&'?'&amp;':char==='<'?'&lt;':'&gt;')}
function md0EditorSetState(text,kind=''){md0EditorState.textContent=text;md0EditorState.className='md0-editor-state '+kind}
function md0SetEditorOpen(open){md0EditorPane.hidden=!open;document.body.classList.toggle('md0-editing',open);md0EditorToggle.querySelector('span').textContent=open?'Close editor':'Edit source';md0SetSettingsOpen(false);if(open){md0EditorRefresh();md0Editor.focus()}}
function md0EditorSetDiagnostic(message){md0EditorDiagnostic.textContent=message||'';md0EditorDiagnostic.hidden=!message}
function md0EditorRememberOverrides(){try{sessionStorage.setItem(md0EditorOverridesKey,JSON.stringify([...md0EditorOverrides]))}catch{}}
function md0EditorDeclaredInputs(){const names=new Set();const pattern=/@input\s+([A-Za-z_][A-Za-z0-9_]*)\s+/g;let match;while((match=pattern.exec(md0Editor.value))!==null)names.add(match[1]);return names}
function md0EditorMarkOverride(event){const input=event.target.closest?.('[data-md0-input]');if(!input||!input.name)return;md0EditorOverrides.add(input.name);md0EditorRememberOverrides()}
function md0EditorInputValues(){const declared=md0EditorDeclaredInputs();const values={};md0EditorOverrides=new Set([...md0EditorOverrides].filter(name=>declared.has(name)));document.querySelectorAll('[data-md0-input]').forEach(el=>{if(!md0EditorOverrides.has(el.name)||!declared.has(el.name))return;values[el.name]=el.type==='checkbox'?String(el.checked):el.value});md0EditorRememberOverrides();return values}

function md0ExpressionHTML(value,plot=false){
 const builtins=plot?md0PlotBuiltins:md0ExpressionBuiltins;
 const pattern=/"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\b(?:true|false|null)\b|\b\d+(?:\.\d+)?(?:[eE][+-]?\d+)?(?:ms|s|m|h)?\b|[A-Za-z_][A-Za-z0-9_]*|&&|\|\||==|!=|<=|>=|[+\-*\/%<>=!?:()[\]{},.]/g;
 let html='';let last=0;let match;
 while((match=pattern.exec(value))!==null){html+=md0Escape(value.slice(last,match.index));const token=match[0];let kind='symbol';if(token[0]==='"'||token[0]==="'")kind='string';else if(/^(true|false|null)$/.test(token))kind='boolean';else if(/^\d/.test(token))kind='number';else if(builtins.has(token))kind='builtin';else if(md0InputTypes.has(token))kind='type';else if(/^[+\-*\/%<>=!?:()[\]{},.&|]+$/.test(token))kind='operator';html+='<span class="tok-'+kind+'">'+md0Escape(token)+'</span>';last=pattern.lastIndex}
 return html+md0Escape(value.slice(last));
}
function md0InlineHTML(value){
 const pattern=/\{\{[\s\S]*?\}\}|\*\*[^*]+\*\*|\[[^\]]+\]\([^)]*\)|\$[^$\n]+\$/g;
 let html='';let last=0;let match;
 while((match=pattern.exec(value))!==null){html+=md0Escape(value.slice(last,match.index));const token=match[0];if(token.startsWith('{{')){html+='<span class="tok-interpolation">{{</span>'+md0ExpressionHTML(token.slice(2,-2))+'<span class="tok-interpolation">}}</span>'}else{html+='<span class="tok-mark">'+md0Escape(token)+'</span>'}last=pattern.lastIndex}
 return html+md0Escape(value.slice(last));
}
function md0HighlightSource(source){
 const lines=source.split('\n');let fence='';let block='';
 return lines.map(line=>{
  const trimmed=line.trimStart();
  if(trimmed.startsWith(md0Fence)){const info=trimmed.slice(3).trim().toLowerCase();if(fence)fence='';else fence=info==='plot'||info==='md0-plot'?'plot':'code';return '<span class="tok-fence">'+md0Escape(line)+'</span>'}
  if(fence==='code')return '<span class="tok-code">'+md0Escape(line)+'</span>';
  if(fence==='plot'){const field=line.match(/^(\s*)([A-Za-z_][A-Za-z0-9_]*\s*\(\s*x\s*\)|title|y[2-4]?|label[2-4]?|x|samples)(\s*=\s*)(.*)$/);if(field)return md0Escape(field[1])+'<span class="tok-key">'+field[2]+'</span><span class="tok-operator">'+md0Escape(field[3])+'</span>'+md0ExpressionHTML(field[4],field[2]!=='title'&&!field[2].startsWith('label'));return md0ExpressionHTML(line,true)}
  const version=line.match(/^(\s*)(md0)(\s*:\s*)(0\.1)(\s*)$/);if(version)return md0Escape(version[1])+'<span class="tok-version">'+version[2]+md0Escape(version[3])+version[4]+'</span>'+md0Escape(version[5]);
  const heading=line.match(/^(\s*)(#{1,6})(\s+)(.*)$/);if(heading)return md0Escape(heading[1])+'<span class="tok-mark">'+heading[2]+'</span>'+md0Escape(heading[3])+'<span class="tok-heading">'+md0InlineHTML(heading[4])+'</span>';
  const directive=line.match(/^(.*?)(@(input|data|calc|show|when|assert|table|chart|end))\b(.*)$/);if(directive){if(directive[3]==='table'||directive[3]==='chart')block=directive[3];else if(directive[3]==='end')block='';return md0InlineHTML(directive[1])+'<span class="tok-directive">'+directive[2]+'</span>'+md0ExpressionHTML(directive[4])}
  if(block){const field=line.match(/^(\s*)(columns|rows|type|labels|values)(\s*=\s*)(.*)$/);if(field)return md0Escape(field[1])+'<span class="tok-key">'+field[2]+'</span><span class="tok-operator">'+md0Escape(field[3])+'</span>'+md0ExpressionHTML(field[4])}
  const list=line.match(/^(\s*)([-+*]|\d+\.)(\s+)(.*)$/);if(list)return md0Escape(list[1])+'<span class="tok-mark">'+list[2]+'</span>'+md0Escape(list[3])+md0InlineHTML(list[4]);
  if(/^\s*&lt;!--/.test(md0Escape(line)))return '<span class="tok-comment">'+md0Escape(line)+'</span>';
  return md0InlineHTML(line);
 }).join('\n');
}
function md0EditorCursor(){const before=md0Editor.value.slice(0,md0Editor.selectionStart);const lines=before.split('\n');return {line:lines.length,column:Array.from(lines[lines.length-1]).length+1,lineIndex:lines.length-1,lineText:lines[lines.length-1]}}
function md0EditorSyncScroll(){document.getElementById('md0-editor-highlight').style.transform='translate('+(-md0Editor.scrollLeft)+'px,'+(-md0Editor.scrollTop)+'px)';md0EditorGutter.style.transform='translateY('+(-md0Editor.scrollTop)+'px)';md0EditorUpdateCurrentLine();md0EditorPlaceCompletions()}
function md0EditorUpdateCurrentLine(){const cursor=md0EditorCursor();const style=getComputedStyle(md0Editor);const top=parseFloat(style.paddingTop)+(cursor.lineIndex*parseFloat(style.lineHeight))-md0Editor.scrollTop;md0EditorCurrentLine.style.top=top+'px';[...md0EditorGutter.children].forEach((line,index)=>line.classList.toggle('active',index===cursor.lineIndex));md0EditorPosition.textContent='Ln '+cursor.line+', Col '+cursor.column}
function md0EditorRefresh(){const lineCount=md0Editor.value.split('\n').length;md0EditorHighlight.innerHTML=md0HighlightSource(md0Editor.value)+'\n';const gutter=document.createDocumentFragment();for(let line=1;line<=lineCount;line++){const span=document.createElement('span');span.textContent=String(line);gutter.appendChild(span)}md0EditorGutter.replaceChildren(gutter);md0EditorSyncScroll()}

function md0EditorSymbols(){const found=new Set();const pattern=/@(input|calc|data)\s+([A-Za-z_][A-Za-z0-9_]*)/g;let match;while((match=pattern.exec(md0Editor.value))!==null)found.add(match[2]);return [...found].sort().map(label=>({label,kind:'symbol',detail:'document value',insert:label}))}
function md0EditorBlockAt(position){const lines=md0Editor.value.slice(0,position).split('\n');let block='';let fence='';for(const line of lines){const trimmed=line.trim();if(trimmed.startsWith(md0Fence)){const info=trimmed.slice(3).trim().toLowerCase();if(fence)fence='';else fence=info==='plot'||info==='md0-plot'?'plot':'code';continue}if(fence)continue;if(/^@table\b/.test(trimmed))block='table';else if(/^@chart\b/.test(trimmed))block='chart';else if(/^@end\b/.test(trimmed))block=''}return fence||block}
function md0EditorCompletionContext(force=false){
 const end=md0Editor.selectionStart;const before=md0Editor.value.slice(0,end);const lineStart=before.lastIndexOf('\n')+1;const line=before.slice(lineStart);const directive=line.match(/@[A-Za-z_]*$/);
 if(directive)return {items:md0DirectiveCompletions,start:end-directive[0].length,end,query:directive[0].toLowerCase()};
 const inputType=line.match(/@input\s+[A-Za-z_][A-Za-z0-9_]*\s+([A-Za-z]*)$/);if(inputType)return {items:md0InputTypeCompletions,start:end-inputType[1].length,end,query:inputType[1].toLowerCase()};
 const dataFormat=line.match(/@data\s+[A-Za-z_][A-Za-z0-9_]*\s+([A-Za-z]*)$/);if(dataFormat)return {items:md0DataFormatCompletions,start:end-dataFormat[1].length,end,query:dataFormat[1].toLowerCase()};
 const block=md0EditorBlockAt(end);const word=line.match(/[A-Za-z_][A-Za-z0-9_]*$/);const start=word?end-word[0].length:end;const query=word?word[0].toLowerCase():'';
 if(block==='plot')return {items:[...md0PlotCompletions,...md0EditorSymbols(),...[...md0PlotBuiltins].sort().map(label=>({label,kind:'function',detail:'plot function',insert:label+'()',cursor:-1}))],start,end,query};
 if(block==='table')return {items:md0TableCompletions,start,end,query};
 if(block==='chart')return {items:md0ChartCompletions,start,end,query};
 const openInterpolation=before.lastIndexOf('{{')>before.lastIndexOf('}}');const expressionLine=/@(calc|show|when|assert)\b/.test(line)||(/@input\b/.test(line)&&line.includes('='));
 const functions=[...md0ExpressionBuiltins].sort().map(label=>({label,kind:'function',detail:'md0 builtin',insert:label+'()',cursor:-1}));
 if(openInterpolation||expressionLine)return {items:[...md0EditorSymbols(),...functions],start,end,query};
 if(force)return {items:[...md0DirectiveCompletions,...md0EditorSymbols(),...functions],start,end,query};
 return null;
}
function md0EditorShowCompletions(force=false){const context=md0EditorCompletionContext(force);if(!context){md0EditorHideCompletions();return}const items=context.items.filter(item=>!context.query||item.label.toLowerCase().startsWith(context.query)).slice(0,9);if(!items.length){md0EditorHideCompletions();return}md0CompletionItems=items;md0CompletionIndex=0;md0CompletionRange={start:context.start,end:context.end};md0EditorRenderCompletions();md0EditorCompletions.hidden=false;md0Editor.setAttribute('aria-expanded','true');md0EditorPlaceCompletions()}
function md0EditorRenderCompletions(){const fragment=document.createDocumentFragment();md0CompletionItems.forEach((item,index)=>{const button=document.createElement('button');button.type='button';button.className='md0-editor-completion'+(index===md0CompletionIndex?' active':'');button.id='md0-editor-option-'+index;button.setAttribute('role','option');button.setAttribute('aria-selected',String(index===md0CompletionIndex));const kind=document.createElement('span');kind.className='md0-editor-kind';kind.textContent=item.kind;const label=document.createElement('code');label.textContent=item.label;const detail=document.createElement('small');detail.textContent=item.detail||'';button.append(kind,label,detail);button.addEventListener('pointerdown',event=>{event.preventDefault();md0CompletionIndex=index;md0EditorAcceptCompletion()});fragment.appendChild(button)});md0EditorCompletions.replaceChildren(fragment);md0Editor.setAttribute('aria-activedescendant','md0-editor-option-'+md0CompletionIndex)}
function md0EditorMoveCompletion(delta){if(md0EditorCompletions.hidden)return;md0CompletionIndex=(md0CompletionIndex+delta+md0CompletionItems.length)%md0CompletionItems.length;md0EditorRenderCompletions();md0EditorCompletions.children[md0CompletionIndex]?.scrollIntoView({block:'nearest'})}
function md0EditorHideCompletions(){md0EditorCompletions.hidden=true;md0Editor.setAttribute('aria-expanded','false');md0Editor.removeAttribute('aria-activedescendant');md0CompletionItems=[]}
function md0EditorAcceptCompletion(){const item=md0CompletionItems[md0CompletionIndex];if(!item)return;const insert=item.insert;md0Editor.setRangeText(insert,md0CompletionRange.start,md0CompletionRange.end,'end');let start=md0CompletionRange.start+insert.length;let end=start;if(item.select){const offset=insert.indexOf(item.select);if(offset>=0){start=md0CompletionRange.start+offset;end=start+item.select.length}}else if(item.cursor){start+=item.cursor;end=start}md0Editor.setSelectionRange(start,end);md0EditorHideCompletions();md0Editor.dispatchEvent(new Event('input',{bubbles:true}));md0Editor.focus()}
function md0EditorPlaceCompletions(){if(md0EditorCompletions.hidden)return;const cursor=md0EditorCursor();const style=getComputedStyle(md0Editor);md0EditorMeasure.textContent=cursor.lineText;const width=md0EditorMeasure.getBoundingClientRect().width;const x=Math.max(62,Math.min(md0Editor.clientWidth-md0EditorCompletions.offsetWidth-12,parseFloat(style.paddingLeft)+width-md0Editor.scrollLeft));const y=Math.max(8,Math.min(md0Editor.clientHeight-md0EditorCompletions.offsetHeight-8,parseFloat(style.paddingTop)+((cursor.lineIndex+1)*parseFloat(style.lineHeight))-md0Editor.scrollTop+4));md0EditorCompletions.style.left=x+'px';md0EditorCompletions.style.top=y+'px'}
function md0EditorInsertText(text){md0Editor.setRangeText(text,md0Editor.selectionStart,md0Editor.selectionEnd,'end');md0Editor.dispatchEvent(new Event('input',{bubbles:true}))}
function md0EditorIndent(outdent=false){const start=md0Editor.selectionStart;const end=md0Editor.selectionEnd;const lineStart=md0Editor.value.lastIndexOf('\n',start-1)+1;if(start===end&&!outdent){md0EditorInsertText('  ');return}const selected=md0Editor.value.slice(lineStart,end);const changed=outdent?selected.replace(/^ {1,2}/gm,''):selected.replace(/^/gm,'  ');md0Editor.setRangeText(changed,lineStart,end,'select');md0Editor.dispatchEvent(new Event('input',{bubbles:true}))}

function md0EditorRequest(){clearTimeout(md0EditorTimer);md0EditorSetState('editing','dirty');md0EditorTimer=setTimeout(md0EditorRenderDraft,180)}
async function md0EditorRenderDraft(){md0EditorQueued=true;if(md0EditorBusy)return;md0EditorBusy=true;try{while(md0EditorQueued){md0EditorQueued=false;let response;try{response=await fetch('/editor/draft',{method:'POST',headers:{'content-type':'application/json','x-md0-editor-token':md0EditorToken},body:JSON.stringify({source:md0Editor.value,values:md0EditorInputValues()})})}catch(err){md0EditorSetState('offline','error');md0EditorSetDiagnostic('md0 editor unavailable: '+err.message);continue}let payload;try{payload=await response.json()}catch{payload={error:'md0 editor returned an invalid response'}}if(!response.ok){md0EditorSetState('invalid','error');md0EditorSetDiagnostic(payload.error||'draft is invalid');continue}const root=document.getElementById('md0-document');root.innerHTML=payload.fragment;md0EnhanceInputs(root);md0EditorSetDiagnostic('');md0EditorSetState(md0EditorDirty?'live · unsaved':'live',md0EditorDirty?'dirty':'ok')}}finally{md0EditorBusy=false}}
async function md0EditorCommit(){md0EditorSetState('saving');let response;try{response=await fetch('/editor/source',{method:'POST',headers:{'content-type':'text/plain; charset=utf-8','x-md0-editor-token':md0EditorToken,'x-md0-source-revision':md0SourceRevision()},body:md0Editor.value})}catch(err){md0EditorSetState('error','error');md0EditorSetDiagnostic('save failed: '+err.message);return}let payload;try{payload=await response.json()}catch{payload={error:'invalid save response'}}if(!response.ok){md0EditorSetState(response.status===409?'conflict':'error','error');md0EditorSetDiagnostic(payload.error||'save failed');return}try{sessionStorage.setItem(md0EditorSelectionKey,JSON.stringify({start:md0Editor.selectionStart,end:md0Editor.selectionEnd,scroll:md0Editor.scrollTop}))}catch{}md0EditorDirty=false;md0EditorSetState('saved','ok');setTimeout(()=>location.reload(),90)}
function md0BeforeSourceReload(){if(!md0EditorDirty)return true;md0EditorSetState('disk changed','error');md0EditorSetDiagnostic('The source changed on disk while this draft has unsaved edits. Copy your draft or reload before saving.');return false}

md0Editor.addEventListener('input',()=>{md0EditorDirty=true;md0EditorRefresh();md0EditorRequest();md0EditorShowCompletions()});
md0Editor.addEventListener('scroll',md0EditorSyncScroll);
md0Editor.addEventListener('click',()=>{md0EditorHideCompletions();md0EditorUpdateCurrentLine()});
md0Editor.addEventListener('keyup',event=>{if(!['ArrowUp','ArrowDown','Enter','Tab','Escape'].includes(event.key)){md0EditorUpdateCurrentLine();if(!md0EditorCompletions.hidden)md0EditorShowCompletions()}});
md0Editor.addEventListener('keydown',event=>{
 if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='s'){event.preventDefault();md0EditorCommit();return}
 if((event.ctrlKey||event.metaKey)&&event.key===' '){event.preventDefault();md0EditorShowCompletions(true);return}
 if(!md0EditorCompletions.hidden&&event.key==='ArrowDown'){event.preventDefault();md0EditorMoveCompletion(1);return}
 if(!md0EditorCompletions.hidden&&event.key==='ArrowUp'){event.preventDefault();md0EditorMoveCompletion(-1);return}
 if(!md0EditorCompletions.hidden&&(event.key==='Enter'||event.key==='Tab')){event.preventDefault();md0EditorAcceptCompletion();return}
 if(event.key==='Escape'){md0EditorHideCompletions();return}
 if(event.key==='Tab'){event.preventDefault();md0EditorIndent(event.shiftKey);return}
 if(event.key==='Enter'){const line=md0Editor.value.slice(0,md0Editor.selectionStart).split('\n').pop()||'';const indent=(line.match(/^\s*/)||[''])[0];if(indent){event.preventDefault();md0EditorInsertText('\n'+indent)}}
});
document.addEventListener('input',md0EditorMarkOverride);
document.addEventListener('change',md0EditorMarkOverride);
md0EditorSave.addEventListener('click',md0EditorCommit);
md0EditorToggle.hidden=false;
md0EditorToggle.addEventListener('click',()=>md0SetEditorOpen(md0EditorPane.hidden));
document.addEventListener('keydown',event=>{if((event.metaKey||event.ctrlKey)&&event.key.toLowerCase()==='e'){event.preventDefault();md0SetEditorOpen(md0EditorPane.hidden)}});
window.addEventListener('resize',()=>{md0EditorSyncScroll();md0EditorPlaceCompletions()});
window.addEventListener('beforeunload',event=>{if(md0EditorDirty){event.preventDefault();event.returnValue=''}});
try{const saved=JSON.parse(sessionStorage.getItem(md0EditorSelectionKey)||'null');sessionStorage.removeItem(md0EditorSelectionKey);if(saved){md0Editor.selectionStart=saved.start||0;md0Editor.selectionEnd=saved.end||saved.start||0;md0Editor.scrollTop=saved.scroll||0;md0Editor.focus()}}catch{}
md0EditorRefresh();
md0SendLatest=md0EditorRenderDraft;
`
