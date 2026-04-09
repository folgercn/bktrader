import re

def main():
    with open('src/main.tsx', 'r') as f:
        content = f.read()

    main_start = content.find('      <main className="main">')
    if main_start == -1:
        print("Could not find <main className=\"main\">")
        return

    main_end = content.find('      </main>', main_start)
    if main_end == -1:
        print("Could not find end of main.")
        return
        
    main_block = content[main_start:main_end + 13]
    original_main_block = main_block

    def extract_by_id(html, id_attr):
        search_str = f'id="{id_attr}"'
        pos = html.find(search_str)
        if pos == -1:
            return None, -1, -1
            
        tag_start = html.rfind('<', 0, pos)
        m = re.match(r'<([a-zA-Z0-9_]+)', html[tag_start:pos])
        if not m:
            return None, -1, -1
        tag_name = m.group(1)
            
        count = 0
        i = tag_start
        while i < len(html):
            if html.startswith(f'<{tag_name}', i) and not html.startswith(f'</{tag_name}>', i):
                if not re.match(r'<[a-zA-Z0-9_]+[^>]*/>', html[i:]):
                    count += 1
            elif html.startswith(f'</{tag_name}>', i):
                count -= 1
                if count == 0:
                    end_pos = i + len(f'</{tag_name}>')
                    return html[tag_start:end_pos], tag_start, end_pos
            i += 1
        return None, -1, -1

    sections = {}
    
    # Extract monitor first, then extract its sub-articles
    sec_monitor, start_m, end_m = extract_by_id(main_block, 'monitor')
    if sec_monitor:
        sub_sections = ['equity', 'positions', 'fills']
        for sid in sub_sections:
            sub_content, s_start, s_end = extract_by_id(sec_monitor, sid)
            if sub_content:
                sections[sid] = sub_content
                sec_monitor = sec_monitor[:s_start] + sec_monitor[s_end:]
            else:
                sections[sid] = f"{{/* missing {sid} */}}"
        sections['monitor'] = sec_monitor
        # remove monitor from main_block
        main_block = main_block[:start_m] + main_block[end_m:]
    else:
        sections['monitor'] = "{/* missing monitor */}"
        for sid in ['equity', 'positions', 'fills']:
            sections[sid] = f"{{/* missing {sid} */}}"

    # Extract other sections from main_block
    for sid in ['overview', 'notifications', 'alerts', 'strategies', 'signals', 'live', 'backtests', 'orders']:
        sec_content, s_start, s_end = extract_by_id(main_block, sid)
        if sec_content:
            sections[sid] = sec_content
            main_block = main_block[:s_start] + main_block[s_end:]
        else:
            sections[sid] = f"{{/* missing {sid} */}}"

    # main_block now contains the modals and `<main className="main">` `</main>` wrapper
    # Remove the <main> wrapper to just get the modals
    main_block = main_block.replace('      <main className="main">\n', '').replace('      </main>', '')
    modals_content = main_block

    # Now replace the parts in main.tsx that correspond to WorkbenchLayout props
    # specifically mainStageContent and dockContent and sidePanelContent
    
    dock_content_start = content.find('dockContent={<div className="p-4 text-zinc-500 text-sm text-center">尚未迁移至新版 Dock...</div>}')
    if dock_content_start == -1:
        print("Could not find dockContent")
        return
        
    main_stage_start = content.find('      mainStageContent={')
    if main_stage_start == -1:
        print("Could not find mainStageContent")
        return
        
    dock_content_str = """      sidePanelContent={
        sidebarTab === 'strategy' ? (
          <div className="p-4 space-y-6">
            {SECTION_BACKTESTS}
          </div>
        ) : sidebarTab === 'account' ? (
          <div className="p-4 space-y-6">
            {SECTION_SIGNALS}
          </div>
        ) : null
      }
      dockContent={
        <div className="h-full">
          <div style={{ display: dockTab === 'orders' ? 'block' : 'none' }} className="h-full p-4">{SECTION_ORDERS}</div>
          <div style={{ display: dockTab === 'positions' ? 'block' : 'none' }} className="h-full p-4 space-y-6">{SECTION_EQUITY}{SECTION_POSITIONS}</div>
          <div style={{ display: dockTab === 'fills' ? 'block' : 'none' }} className="h-full p-4">{SECTION_FILLS}</div>
          <div style={{ display: dockTab === 'alerts' ? 'block' : 'none' }} className="h-full p-4 space-y-6">{SECTION_ALERTS}{SECTION_NOTIFICATIONS}</div>
        </div>
      }"""
      
    main_stage_str = """      mainStageContent={
        sidebarTab === 'monitor' ? (
          <div className="absolute inset-0 flex flex-col p-4 bg-zinc-950/50">
            {SECTION_MONITOR}
          </div>
        ) : sidebarTab === 'strategy' ? (
          <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
            {SECTION_STRATEGIES}
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
            {SECTION_OVERVIEW}
            {SECTION_LIVE}
          </div>
        )
      }"""

    for k, v in sections.items():
        dock_content_str = dock_content_str.replace(f'{{SECTION_{k.upper()}}}', v)
        main_stage_str = main_stage_str.replace(f'{{SECTION_{k.upper()}}}', v)
        
    main_stage_end = content.find('        </div>\n      }\n    />', main_stage_start)
    if main_stage_end == -1:
        print("Could not find end of mainStageContent")
        return
        
    new_content = content[:dock_content_start] + dock_content_str + "\n" + main_stage_str + "\n    />\n" + modals_content + "\n    </>\n" + content[main_stage_end + len('        </div>\n      }\n    />'):]
    
    # We need to prepend `<>` right before `<WorkbenchLayout`
    start_return = new_content.find('  return (\n    <WorkbenchLayout')
    if start_return != -1:
        new_content = new_content[:start_return + len('  return (\n')] + '    <>\n' + new_content[start_return + len('  return (\n'):]

    with open('src/main.tsx', 'w') as f:
        f.write(new_content)

    print("Waterfall distributed successfully with modals kept.")

if __name__ == '__main__':
    main()
