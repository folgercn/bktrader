import re

def refactor_modal(content, condition_var, title, width_class="w-full max-w-md"):
    # Find the modal pattern
    # It usually looks like:
    # {condition_var ? (
    #   <div className="modal-overlay" onClick={() => ...}>
    #     <div className="modal-content" onClick={(e) => e.stopPropagation()}>
    #       <div className="panel-header">
    #         <div>
    #           <p className="panel-kicker">...</p>
    #           <h3>...</h3>
    #         </div>
    #         <button ... onClick={() => ...}>关闭</button>
    #       </div>
    #       <div className="backtest-form modal-form">
    #         ... form content ...
    #       </div>
    #     </div>
    #   </div>
    # ) : null}
    
    # Let's search for the start:
    start_str = f'        {{{condition_var} ? ('
    start_idx = content.find(start_str)
    if start_idx == -1:
        # try without indentation
        start_idx = content.find(f'{{{condition_var} ? (')
        if start_idx == -1:
            print(f"Could not find modal for {condition_var}")
            return content
            
    # Find the matching `: null}`
    # Count braces or just regex.
    # Since it's a simple ternary at the top level, we can count braces and parentheses.
    i = start_idx
    paren_count = 0
    in_paren = False
    for j in range(start_idx, len(content)):
        if content[j] == '?' and content[j+1:j+3] == ' (':
            in_paren = True
            paren_count = 1
            i = j + 3
            break
            
    if not in_paren:
        return content
        
    end_idx = -1
    for j in range(i, len(content)):
        if content[j] == '(':
            paren_count += 1
        elif content[j] == ')':
            paren_count -= 1
            if paren_count == 0:
                # expecting ` : null}`
                next_text = content[j:j+20]
                if 'null}' in next_text:
                    end_idx = j + next_text.find('null}') + 5
                    break

    if end_idx == -1:
        print(f"Could not find end of modal for {condition_var}")
        return content
        
    modal_block = content[start_idx:end_idx]
    
    # Extract the close function from onClick
    # onClick={() => setActiveSettingsModal(null)}
    m_close = re.search(r'onClick=\{\(\) => ([^\}]+)\}', modal_block)
    on_close = m_close.group(1) if m_close else "() => {}"
    
    # Extract the form content.
    # It's usually inside `<div className="backtest-form modal-form">` or `<div className="form-grid">`
    # Let's extract everything inside `<div className="modal-content"...>` but skip the `<div className="panel-header">`
    
    # Find modal-content
    mc_start = modal_block.find('<div className="modal-content"')
    if mc_start == -1:
        # maybe another class?
        mc_start = modal_block.find('<div className="modal-panel"')
        if mc_start == -1:
            mc_start = modal_block.find('<div className="panel modal-content"')
            if mc_start == -1:
                mc_start = modal_block.find('<div className="panel')
            
    if mc_start == -1:
        print(f"Could not find modal-content for {condition_var}")
        return content
        
    # Find the inner children of modal-content
    # It starts after `<div className="...">`
    mc_inner_start = modal_block.find('>', mc_start) + 1
    
    # Find the matching closing tag of modal-content
    count = 1
    mc_inner_end = -1
    for j in range(mc_inner_start, len(modal_block)):
        if modal_block.startswith('<div', j):
            count += 1
        elif modal_block.startswith('</div', j):
            count -= 1
            if count == 0:
                mc_inner_end = j
                break
                
    content_inside = modal_block[mc_inner_start:mc_inner_end]
    
    # Now remove the panel-header from content_inside
    ph_start = content_inside.find('<div className="panel-header')
    if ph_start != -1:
        # Find its end
        count = 1
        ph_inner_start = content_inside.find('>', ph_start) + 1
        ph_end = -1
        for j in range(ph_inner_start, len(content_inside)):
            if content_inside.startswith('<div', j):
                count += 1
            elif content_inside.startswith('</div', j):
                count -= 1
                if count == 0:
                    ph_end = j + 6
                    break
        if ph_end != -1:
            content_inside = content_inside[:ph_start] + content_inside[ph_end:]
            
    # Now wrap it in SlideOver
    is_open_expr = f"{condition_var}"
    if '===' in condition_var:
        is_open_expr = condition_var
        
    slide_over = f"""      <SlideOver
        isOpen={{{is_open_expr}}}
        onClose={{() => {on_close}}}
        title="{title}"
        widthClass="{width_class}"
      >
{content_inside}
      </SlideOver>"""

    return content[:start_idx] + slide_over + content[end_idx:]


def main():
    with open('src/main.tsx', 'r') as f:
        content = f.read()

    content = refactor_modal(content, 'activeSettingsModal === "live-account"', '交易所 / 经纪商账户')
    content = refactor_modal(content, 'activeSettingsModal === "live-binding"', '实盘环境配置 (API Keys)')
    content = refactor_modal(content, 'activeSettingsModal === "live-session"', '运行实盘会话')
    content = refactor_modal(content, 'activeSettingsModal === "telegram"', 'Telegram Bot 告警配置')
    
    # For auth, let's also use SlideOver? Or keep it modal but style it?
    # SlideOver is fine.
    content = refactor_modal(content, '!authSession', '登录平台 API')
    
    if 'import { SlideOver }' not in content:
        import_idx = content.find('import ')
        content = content[:import_idx] + "import { SlideOver } from './components/SlideOver';\n" + content[import_idx:]

    with open('src/main.tsx', 'w') as f:
        f.write(content)

    print("Modals refactored.")

if __name__ == '__main__':
    main()
