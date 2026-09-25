const {test}=require("node:test");
const assert=require("node:assert/strict");
const fs=require("node:fs");
const path=require("node:path");
const vm=require("node:vm");

const source=fs.readFileSync(path.join(__dirname,"..","static/js/admin-book-submissions.js"),"utf8");

function setup(results) {
  let ready;
  const refresh={addEventListener() {}};
  const notice={textContent:""};
  const body={
    rows:[],
    replaceChildren(){this.rows=[];},
    addEventListener(){},
    insertRow(){
      const row={cells:[],insertCell(){
        const cell={children:[],textContent:"",append(...values){this.children.push(...values);}};
        this.cells.push(cell);
        return cell;
      }};
      this.rows.push(row);
      return row;
    }
  };
  const retryDialog={
    querySelector(){return {addEventListener(){},focus(){}};},
    querySelectorAll(){return [];},
    addEventListener(){}
  };
  const panel={querySelector(selector){
    if(selector==="tbody") return body;
    if(selector==="button") return refresh;
    if(selector==="[data-submission-notice]") return notice;
    return null;
  }};
  const document={
    addEventListener(event, listener){if(event==="DOMContentLoaded") ready=listener;},
    querySelector(selector){
      if(selector==="#book-submissions-panel") return panel;
      if(selector==="#submission-retry-dialog") return retryDialog;
      return null;
    },
    createElement(){return {dataset:{},className:"",textContent:"",title:"",append(){}};}
  };
  const window={
    DeftaHTTP:{
      json:async()=>({role:"SUPER_ADMIN_ROOT"}),
      request:async()=>({json:async()=>({results})})
    }
  };
  vm.runInNewContext(source,{window,document,encodeURIComponent,JSON});
  return {body,ready};
}

test("moderation statuses and decisions use readable semantic pills",async()=>{
  const {body,ready}=setup([
    {id:"1",title:"Sûr",moderationStatus:"APPROVED",moderationScore:.04,decisionCode:"MODEL_SAFE",createdBookId:12},
    {id:"2",title:"Refusé",moderationStatus:"REJECTED",moderationScore:.78,decisionCode:"MODEL_UNSAFE"},
    {id:"3",title:"Relancé",moderationStatus:"PENDING_SCAN",decisionCode:"RETRY_REQUESTED"},
    {id:"4",title:"À revoir",moderationStatus:"REVIEW_REQUIRED"}
  ]);
  await ready();

  const approved=body.rows[0].cells;
  assert.equal(approved[1].children[0].textContent,"Approuvée");
  assert.match(approved[1].children[0].className,/success/);
  assert.equal(approved[3].children[0].textContent,"Contenu conforme");

  const rejected=body.rows[1].cells;
  assert.equal(rejected[1].children[0].textContent,"Refusée");
  assert.match(rejected[1].children[0].className,/danger/);
  assert.equal(rejected[3].children[0].textContent,"Contenu non conforme");

  const pending=body.rows[2].cells;
  assert.equal(pending[1].children[0].textContent,"En attente d’analyse");
  assert.equal(pending[3].children[0].textContent,"Nouvelle analyse demandée");

  const review=body.rows[3].cells;
  assert.equal(review[1].children[0].textContent,"Revue manuelle requise");
  assert.equal(review[3].textContent,"—");
});
