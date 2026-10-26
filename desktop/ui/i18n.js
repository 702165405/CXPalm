(function(root,factory){
  if(typeof module==='object'&&module.exports)module.exports=factory();else root.CXPalmI18n=factory();
})(globalThis,()=>{
  const valid=value=>value==='zh-CN'||value==='en';
  const choose=value=>valid(value)?value:String(value||'').toLowerCase().startsWith('zh')?'zh-CN':'en';
  const fill=(value,args)=>value.replace(/\{(\d+)\}/g,(token,index)=>index<args.length?String(args[index]):token);
  const escape=value=>value.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
  function create(messages,initial){
    let language=choose(initial);
    const reverse=new Map(),patterns=[];
    for(const [zh,en] of Object.entries(messages)){
      if(!reverse.has(en))reverse.set(en,zh);
      if(/\{\d+\}/.test(zh))for(const [from,to] of [[zh,en],[en,zh]]){
        const slots=[];let at=0,source='^';
        for(const match of from.matchAll(/\{(\d+)\}/g)){source+=escape(from.slice(at,match.index))+'([\\s\\S]*?)';slots.push(Number(match[1]));at=match.index+match[0].length;}
        patterns.push({from,to,zh,source:new RegExp(source+escape(from.slice(at))+'$'),slots});
      }
    }
    function tr(value,...args){
      if(typeof value!=='string')return value;
      const canonical=Object.hasOwn(messages,value)?value:reverse.get(value);
      if(canonical!==undefined)return fill(language==='en'?messages[canonical]:canonical,args);
      if(args.length)return fill(value,args);
      for(const pattern of patterns){const match=pattern.source.exec(value);if(match){const values=[];pattern.slots.forEach((slot,index)=>values[slot]=match[index+1]);return fill(language==='en'?messages[pattern.zh]:pattern.zh,values);}}
      return value;
    }
    return {tr,get code(){return language;},set:value=>{if(!valid(value))throw new Error('Invalid language');language=value;},locale:()=>language==='en'?'en-US':'zh-CN'};
  }
  return {valid,choose,create};
});
