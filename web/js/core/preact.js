/**
 * Preact + HTM bridge module
 *
 * Provides Preact hooks and HTM tagged template for all modules.
 * Must be loaded after vendor/preact.min.js, vendor/preact-hooks.min.js, and vendor/htm.min.js.
 *
 * Usage in modules:
 *   var html = PreactBridge.html;
 *   var useState = PreactBridge.useState;
 *   var h = PreactBridge.h;
 */
(function() {
  'use strict';

  var h = preact.h;
  var render = preact.render;
  var Component = preact.Component;
  var Fragment = preact.Fragment;
  var createElement = preact.createElement;
  var createContext = preact.createContext;
  var cloneElement = preact.cloneElement;
  var toChildArray = preact.toChildArray;
  var useState = preactHooks.useState;
  var useEffect = preactHooks.useEffect;
  var useReducer = preactHooks.useReducer;
  var useRef = preactHooks.useRef;
  var useCallback = preactHooks.useCallback;
  var useMemo = preactHooks.useMemo;
  var useContext = preactHooks.useContext;
  var useLayoutEffect = preactHooks.useLayoutEffect;
  var useDebugValue = preactHooks.useDebugValue;

  // Bind HTM to Preact's h function for tagged template literals
  var html = htm.bind(h);

  // Expose as global bridge object
  window.PreactBridge = {
    h: h,
    html: html,
    render: render,
    Component: Component,
    Fragment: Fragment,
    createElement: createElement,
    createContext: createContext,
    cloneElement: cloneElement,
    toChildArray: toChildArray,
    useState: useState,
    useEffect: useEffect,
    useReducer: useReducer,
    useRef: useRef,
    useCallback: useCallback,
    useMemo: useMemo,
    useContext: useContext,
    useLayoutEffect: useLayoutEffect,
    useDebugValue: useDebugValue
  };
})();
