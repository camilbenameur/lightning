interface NavigatorUAData {
  userAgentData?: { platform: string };
}

const platform = (navigator as NavigatorUAData).userAgentData?.platform ?? navigator.userAgent;

/** Label of the primary shortcut modifier: ⌘ on Apple platforms, Ctrl elsewhere. */
export const MOD_KEY = /mac|iphone|ipad|ipod/i.test(platform) ? '⌘' : 'Ctrl';
