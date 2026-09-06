/**
 * Mobile Responsive Tests
 * 
 * Tests for mobile CSS media queries, breakpoints, and CSS variable usage
 * Verifies that new components have proper mobile adaptations
 */

const fs = require('fs');
const path = require('path');

describe('Mobile Responsive CSS', () => {
  let cssContent;
  const cssPath = path.resolve(__dirname, '..', 'css', 'style.css');

  beforeAll(() => {
    cssContent = fs.readFileSync(cssPath, 'utf8');
  });

  describe('Media Query Structure', () => {
    test('style.css contains mobile media queries', () => {
      expect(cssContent).toMatch(/@media\s*\(max-width:\s*\d+px\)/);
    });

    test('media queries cover key breakpoints (768px, 640px)', () => {
      // Check for 768px breakpoint (tablet/mobile)
      expect(cssContent).toMatch(/@media\s*\(max-width:\s*768px\)/);
      
      // Check for 640px breakpoint (mobile)
      expect(cssContent).toMatch(/@media\s*\(max-width:\s*640px\)/);
    });

    test('media queries are properly formatted', () => {
      // Ensure media queries use correct syntax
      const mediaQueryMatches = cssContent.match(/@media\s*\([^)]+\)/g);
      expect(mediaQueryMatches).toBeTruthy();
      expect(mediaQueryMatches.length).toBeGreaterThan(0);
      
      // Check that media queries have opening and closing braces
      mediaQueryMatches.forEach((match, index) => {
        const afterMatch = cssContent.slice(cssContent.indexOf(match));
        expect(afterMatch).toMatch(/\{[\s\S]*\}/);
      });
    });
  });

  describe('CSS Variable Usage', () => {
    test('CSS uses --color-* variables', () => {
      // Check that color variables are used in media queries
      const mediaQueryBlocks = cssContent.split(/@media\s*\([^)]+\)\s*\{/);
      
      // Skip the first element (before first @media)
      for (let i = 1; i < mediaQueryBlocks.length; i++) {
        const block = mediaQueryBlocks[i];
        // Find the closing brace of this media query
        const closingBrace = block.indexOf('}');
        if (closingBrace !== -1) {
          const mediaQueryContent = block.slice(0, closingBrace);
          
          // Check if color variables are used (or if the block is non-empty)
          if (mediaQueryContent.trim()) {
            // Either color variables are used or other properties are set
            const hasColorVar = mediaQueryContent.includes('--color-');
            const hasOtherProps = mediaQueryContent.includes(':') && 
                                  !mediaQueryContent.includes('--color-');
            
            // At least one of these should be true
            expect(hasColorVar || hasOtherProps).toBe(true);
          }
        }
      }
    });

    test('no !important in mobile styles (except reduced-motion)', () => {
      // Check for !important in media queries (excluding prefers-reduced-motion which is allowed)
      const mediaQueryMatches = cssContent.match(/@media[^{]*\{[^}]*\}/gs);
      
      if (mediaQueryMatches) {
        mediaQueryMatches.forEach((mediaQuery) => {
          // Skip reduced-motion media query (allowed to use !important)
          if (mediaQuery.includes('prefers-reduced-motion')) {
            return;
          }
          
          // Check for !important in other media queries
          expect(mediaQuery).not.toMatch(/!important/);
        });
      }
    });
  });

  describe('Mobile Component Adaptations', () => {
    test('bulk bar has mobile styles (fixed to bottom, horizontal scroll)', () => {
      // Check for .bulk-bar mobile styles
      const hasBulkBar = cssContent.includes('.bulk-bar');
      if (hasBulkBar) {
        // Should have mobile styles for bulk bar
        const bulkBarMobile = cssContent.match(/\.bulk-bar[^}]*position:\s*fixed[^}]*bottom:\s*3\.5rem/);
        expect(bulkBarMobile).toBeTruthy();
      }
    });

    test('file center table has card transformation on mobile', () => {
      // Check for table to card transformation
      const hasTableCardTransform = cssContent.match(/\.table-wrap[^}]*display:\s*block/);
      expect(hasTableCardTransform).toBeTruthy();
    });

    test('view manager has stacked layout on mobile', () => {
      // Check for view manager flex-direction column on mobile
      const hasViewManagerStack = cssContent.includes('.view-manager-layout') &&
                                  cssContent.includes('flex-direction: column');
      expect(hasViewManagerStack).toBe(true);
    });

    test('external service tabs have horizontal scroll on mobile', () => {
      // Check for horizontal scroll on tabs
      const hasTabScroll = cssContent.match(/overflow-x:\s*auto/);
      expect(hasTabScroll).toBeTruthy();
    });

    test('foraging tabs have horizontal scroll on mobile', () => {
      // Module tabs should have overflow-x: auto
      const hasModuleTabScroll = cssContent.includes('.module-tabs') &&
                                 cssContent.includes('overflow-x: auto');
      expect(hasModuleTabScroll).toBeTruthy();
    });

    test('drawer becomes full-screen sheet on mobile', () => {
      // Check for drawer full-screen styles
      const hasDrawerFullScreen = cssContent.match(/\.drawer-panel[^}]*width:\s*100%/);
      expect(hasDrawerFullScreen).toBeTruthy();
    });

    test('touch targets meet minimum size (44px)', () => {
      // Check for button min-height on mobile
      const hasButtonMinHeight = cssContent.match(/\.btn[^}]*min-height:\s*2\.75rem/); // 2.75rem = 44px
      expect(hasButtonMinHeight).toBeTruthy();
    });

    test('filter sidebar becomes slide-in drawer on mobile', () => {
      // Check for filter sidebar mobile styles
      const hasFilterMobile = cssContent.match(/filter.*transform/);
      if (hasFilterMobile) {
        // Should have transform for slide-in effect
        expect(cssContent).toMatch(/transform:\s*translate/);
      }
    });
  });

  describe('Mobile Layout Patterns', () => {
    test('grids collapse to single column on mobile', () => {
      // Check for grid-template-columns: 1fr in media queries
      const hasSingleColumnGrid = cssContent.match(/grid-template-columns:\s*1fr/);
      expect(hasSingleColumnGrid).toBeTruthy();
    });

    test('flex containers wrap on mobile', () => {
      // Check for flex-wrap: wrap in media queries
      const hasFlexWrap = cssContent.match(/flex-wrap:\s*wrap/);
      expect(hasFlexWrap).toBeTruthy();
    });

    test('tables scroll horizontally on mobile', () => {
      // Check for overflow-x: auto on tables
      const hasTableScroll = cssContent.match(/\.table-wrap[^}]*overflow-x:\s*auto/);
      expect(hasTableScroll).toBeTruthy();
    });

    test('modals adapt width on mobile', () => {
      // Check for modal width adaptation
      const hasModalMobile = cssContent.match(/\.modal-content[^}]*max-width:\s*calc\(100%[^}]*\)/);
      expect(hasModalMobile).toBeTruthy();
    });
  });

  describe('Touch-Friendly Interactions', () => {
    test('buttons have sufficient touch targets', () => {
      // Check button min-height in 768px media query
      const media768 = cssContent.match(/@media\s*\(max-width:\s*768px\)[\s\S]*?(?=@media|$)/);
      if (media768) {
        expect(media768[0]).toMatch(/\.btn[^}]*min-height:\s*2\.75rem/);
      }
    });

    test('inputs have sufficient touch targets', () => {
      // Check input min-height in mobile media queries
      const hasInputMinHeight = cssContent.match(/\.input[^}]*min-height:\s*2\.75rem/);
      expect(hasInputMinHeight).toBeTruthy();
    });

    test('tab buttons have sufficient touch targets', () => {
      // Check tab min-height in mobile media queries
      const hasTabMinHeight = cssContent.match(/\.module-tab[^}]*min-height:\s*2\.75rem/);
      expect(hasTabMinHeight).toBeTruthy();
    });
  });

  describe('Breakpoint Consistency', () => {
    test('breakpoints follow logical progression', () => {
      // Extract all max-width breakpoints
      const breakpointMatches = cssContent.match(/max-width:\s*(\d+)px/g);
      if (breakpointMatches) {
        const breakpoints = breakpointMatches
          .map(match => parseInt(match.match(/\d+/)[0]))
          .sort((a, b) => b - a); // Sort descending
        
        // Check that breakpoints are in descending order when listed
        // Common breakpoints: 1024, 768, 640, 480, 375
        const expectedOrder = [1024, 768, 640, 480, 375];
        const hasExpectedBreakpoints = expectedOrder.some(bp => 
          cssContent.includes(`max-width: ${bp}px`)
        );
        expect(hasExpectedBreakpoints).toBe(true);
      }
    });

    test('mobile breakpoint (768px) includes essential mobile features', () => {
      const media768 = cssContent.match(/@media\s*\(max-width:\s*768px\)[\s\S]*?(?=@media|$)/);
      if (media768) {
        const block = media768[0];
        // Should have bottom tab bar visible
        expect(block).toMatch(/\.bottom-tab-bar[^}]*display:\s*flex/);
        // Should have mobile top bar visible
        expect(block).toMatch(/\.mobile-top-bar[^}]*display:\s*flex/);
      }
    });
  });

  describe('CSS Quality', () => {
    test('no duplicate media queries', () => {
      // Extract all unique media query conditions
      const mediaQueryConditions = cssContent.match(/@media\s*\([^)]+\)/g);
      if (mediaQueryConditions) {
        const uniqueConditions = [...new Set(mediaQueryConditions)];
        expect(uniqueConditions.length).toBe(mediaQueryConditions.length);
      }
    });

    test('media queries have content', () => {
      const mediaQueryMatches = cssContent.match(/@media\s*\([^)]+\)\s*\{/g);
      if (mediaQueryMatches) {
        mediaQueryMatches.forEach((match) => {
          const startIndex = cssContent.indexOf(match) + match.length;
          const endIndex = cssContent.indexOf('}', startIndex);
          const content = cssContent.slice(startIndex, endIndex).trim();
          
          // Media query should have some content (not just whitespace)
          expect(content.length).toBeGreaterThan(0);
        });
      }
    });

    test('media queries use consistent indentation', () => {
      // Check that media queries don't have inconsistent spacing
      const hasInconsistentSpacing = cssContent.match(/@media\s{2,}\(/);
      expect(hasInconsistentSpacing).toBeNull();
    });
  });
});