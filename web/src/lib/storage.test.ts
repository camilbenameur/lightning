import { describe, expect, it, vi } from 'vitest';
import { loadDraft, readItem, saveDraft, writeItem } from './storage';

describe('storage', () => {
  it('namespaces keys', () => {
    writeItem('theme', 'dark');
    expect(window.localStorage.getItem('lightning:theme')).toBe('dark');
    expect(readItem('theme')).toBe('dark');
  });

  it('round-trips drafts per language', () => {
    saveDraft('python', { code: 'print(1)', stdin: 'in' });
    saveDraft('go', { code: 'package main', stdin: '' });

    expect(loadDraft('python')).toEqual({ code: 'print(1)', stdin: 'in' });
    expect(loadDraft('go')).toEqual({ code: 'package main', stdin: '' });
    expect(loadDraft('rust')).toBeNull();
  });

  it('ignores corrupted drafts', () => {
    window.localStorage.setItem('lightning:draft:python', '{oops');
    window.localStorage.setItem('lightning:draft:go', JSON.stringify({ code: 1 }));
    window.localStorage.setItem('lightning:draft:c', JSON.stringify({ code: 'x' }));

    expect(loadDraft('python')).toBeNull();
    expect(loadDraft('go')).toBeNull();
    expect(loadDraft('c')).toEqual({ code: 'x', stdin: '' });
  });

  it('degrades gracefully when storage is unavailable', () => {
    const fail = () => {
      throw new DOMException('Denied', 'SecurityError');
    };
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(fail);
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(fail);

    expect(() => {
      saveDraft('python', { code: 'x', stdin: '' });
    }).not.toThrow();
    expect(loadDraft('python')).toBeNull();
    expect(readItem('theme')).toBeNull();
  });
});
