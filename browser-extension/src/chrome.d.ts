type ChromeTab = {
  id?: number;
  title?: string;
  url?: string;
};

type ChromeMessageSender = {
  tab?: ChromeTab;
};

declare const chrome: {
  runtime: {
    lastError?: { message?: string };
    onMessage: {
      addListener(
        callback: (
          message: unknown,
          sender: ChromeMessageSender,
          sendResponse: (response: unknown) => void,
        ) => boolean | void,
      ): void;
    };
    sendMessage<T = unknown>(message: unknown): Promise<T>;
  };
  sidePanel: {
    setPanelBehavior(options: { openPanelOnActionClick: boolean }): Promise<void>;
  };
  scripting: {
    executeScript<T>(options: {
      target: { tabId: number };
      func: () => T | Promise<T>;
    }): Promise<Array<{ result?: T }>>;
  };
  tabs: {
    query(options: { active: boolean; lastFocusedWindow?: boolean; currentWindow?: boolean }): Promise<ChromeTab[]>;
    create(options: { url: string }): Promise<ChromeTab>;
    onActivated: {
      addListener(callback: () => void): void;
    };
    onUpdated: {
      addListener(callback: (tabId: number, changeInfo: { status?: string; url?: string }) => void): void;
    };
  };
  storage: {
    local: {
      get(keys?: string | string[]): Promise<Record<string, unknown>>;
      set(items: Record<string, unknown>): Promise<void>;
      remove(keys: string | string[]): Promise<void>;
    };
  };
  permissions: {
    contains(permissions: { origins: string[] }): Promise<boolean>;
    request(permissions: { origins: string[] }): Promise<boolean>;
  };
};
