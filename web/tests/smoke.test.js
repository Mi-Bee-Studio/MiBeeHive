/**
 * Smoke tests — verify test infrastructure loads correctly.
 */
describe('Test infrastructure', () => {
  it('PreactBridge.html is callable', () => {
    expect(typeof window.PreactBridge.html).toBe('function');
    // html is a tagged template tag bound to Preact's h
    const result = window.PreactBridge.html`<div>hello</div>`;
    expect(result).toBeDefined();
  });

  it('t() returns a string', () => {
    const result = t('some_key');
    expect(typeof result).toBe('string');
    expect(result).toBe('some_key');
  });

  it('Api.get is a function', () => {
    expect(typeof Api.get).toBe('function');
  });
});
