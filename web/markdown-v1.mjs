import {Marked} from './marked-vendor.mjs';
import DOMPurify from './purify-vendor.mjs';

const escape = value => value.replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const parser = new Marked({gfm:true,breaks:true,async:false,renderer:{
  // Model output is Markdown, never executable HTML or remote image requests.
  html:({text})=>escape(text),
  image:({text})=>escape(text || '图片'),
}});
export function markdown(body, value) {
  const fragment=DOMPurify.sanitize(parser.parse(value),{
    RETURN_DOM_FRAGMENT:true,
    ALLOWED_TAGS:['p','br','strong','em','del','s','blockquote','ul','ol','li','h1','h2','h3','h4','h5','h6','pre','code','hr','table','thead','tbody','tr','th','td','a','input'],
    ALLOWED_ATTR:['href','title','class','start','align','type','checked','disabled'],
    ALLOW_DATA_ATTR:false,ALLOW_ARIA_ATTR:false,
  });
  for(const link of fragment.querySelectorAll('a')){
    const href=link.getAttribute('href') || '';
    if(!/^(?:https?:\/\/|mailto:|#)/i.test(href)){link.replaceWith(...link.childNodes);continue;}
    link.target='_blank';link.rel='noopener noreferrer';link.referrerPolicy='no-referrer';
  }
  for(const input of fragment.querySelectorAll('input')){input.type='checkbox';input.disabled=true;input.tabIndex=-1;}
  for(const pre of fragment.querySelectorAll('pre')){
    const language=pre.querySelector('code')?.className.match(/(?:^|\s)language-([\w+-]+)/)?.[1];
    if(language){const caption=document.createElement('div');caption.className='code-caption';caption.textContent=language.slice(0,80);pre.before(caption);}
    pre.tabIndex=0;
  }
  for(const table of fragment.querySelectorAll('table')){
    const scroll=document.createElement('div');scroll.className='markdown-table';scroll.tabIndex=0;scroll.setAttribute('role','region');scroll.setAttribute('aria-label','表格，可横向滚动');table.before(scroll);scroll.append(table);
  }
  body.append(fragment);
}
