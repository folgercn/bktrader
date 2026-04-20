import { buildTimelineNotes } from '../derivation';

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(`Assertion Failed: ${message}`);
}

function expect(actual: any) {
  return {
    toBe(expected: any, message = '') {
      if (actual !== expected) throw new Error(`Expected ${expected}, got ${actual}. ${message}`);
    },
    toContain(expected: string, message = '') {
      if (!String(actual).includes(expected)) throw new Error(`Expected content to contain ${expected}. ${message}`);
    },
    toBeLength(expected: number, message = '') {
      if (!Array.isArray(actual) || actual.length !== expected) {
        throw new Error(`Expected length ${expected}, got ${actual?.length}. ${message}`);
      }
    }
  };
}

const createItem = (id: string, category: string, title: string, time: string, metadata = {}) => ({
  id,
  category,
  title,
  time,
  metadata,
});

const mockConfig = {
  deduplicationEnabled: true,
  quietSeconds: 60,
  maxRepeats: 1,
};

async function runTests() {
  console.log('🚀 Running buildTimelineNotes Deduplication Tests...');

  // 1. 基础过滤测试
  {
    const items = [
      createItem('1', 'strategy', 'decision-wait', '2026-04-20T10:00:00Z', { reason: 'bias-unfavorable' }),
      createItem('2', 'strategy', 'decision-wait', '2026-04-20T10:00:30Z', { reason: 'bias-unfavorable' }),
    ];
    const notes = buildTimelineNotes(items, mockConfig, 'sess-1');
    expect(notes).toBeLength(1, '应该在 60s 窗口内过滤重复项');
    console.log('✅ 基础过滤通过');
  }

  // 2. 交错过滤测试 (A -> B -> A)
  {
    const items = [
      createItem('1', 'strategy', 'decision-wait', '2026-04-20T10:00:00Z', { reason: 'A' }),
      createItem('2', 'strategy', 'decision-wait', '2026-04-20T10:00:10Z', { reason: 'B' }),
      createItem('3', 'strategy', 'decision-wait', '2026-04-20T10:00:20Z', { reason: 'A' }),
    ];
    const notes = buildTimelineNotes(items, mockConfig, 'sess-1');
    expect(notes).toBeLength(2, '交错出现的重复 A 应因命中 Map 记录而被过滤');
    expect(notes[0]).toContain('B');
    expect(notes[1]).toContain('A');
    console.log('✅ 交错过滤通过');
  }

  // 3. 窗口到期测试
  {
    const items = [
      createItem('1', 'strategy', 'decision-wait', '2026-04-20T10:00:00Z', { reason: 'wait' }),
      createItem('2', 'strategy', 'decision-wait', '2026-04-20T10:01:01Z', { reason: 'wait' }),
    ];
    const notes = buildTimelineNotes(items, mockConfig, 'sess-1');
    expect(notes).toBeLength(2, '超过 quietSeconds 后应重置窗口');
    console.log('✅ 窗口到期重置通过');
  }

  // 4. maxRepeats 语义测试
  {
    // maxRepeats=2 表示窗口内允许总计显示 2 次
    const multiConfig = { ...mockConfig, maxRepeats: 2 };
    const items = [
      createItem('1', 'strategy', 'decision-wait', '2026-04-20T10:00:00Z', { reason: 'wait' }),
      createItem('2', 'strategy', 'decision-wait', '2026-04-20T10:00:10Z', { reason: 'wait' }),
      createItem('3', 'strategy', 'decision-wait', '2026-04-20T10:00:20Z', { reason: 'wait' }),
    ];
    const notes = buildTimelineNotes(items, multiConfig, 'sess-1');
    expect(notes).toBeLength(2, 'maxRepeats=2 应允许显示前 2 条');
    console.log('✅ maxRepeats 语义通过');
  }

  // 5. 会话隔离测试
  {
    const items = [
      createItem('1', 'strategy', 'decision-wait', '2026-04-20T10:00:00Z', { reason: 'wait', liveSessionId: 's1' }),
      createItem('2', 'strategy', 'decision-wait', '2026-04-20T10:00:01Z', { reason: 'wait', liveSessionId: 's2' }),
    ];
    const notes = buildTimelineNotes(items, mockConfig);
    expect(notes).toBeLength(2, '不同会话 ID 不应互相合并');
    console.log('✅ 会话隔离通过');
  }

  // 6. 优先级豁免测试
  {
    const items = [
      createItem('1', 'alert', 'risk', '2026-04-20T10:00:00Z'),
      createItem('2', 'alert', 'risk', '2026-04-20T10:00:05Z'),
    ];
    const notes = buildTimelineNotes(items, mockConfig);
    expect(notes).toBeLength(2, 'ALERT/EXECUTION 级别不应被过滤');
    console.log('✅ 优先级豁免通过');
  }

  console.log('🎉 All tests passed successfully!');
}

runTests().catch(err => {
  console.error('❌ Test execution failed:');
  console.error(err);
  process.exit(1);
});
