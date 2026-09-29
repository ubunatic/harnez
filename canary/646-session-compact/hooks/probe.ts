const MARKER = 'HARNEZ_646_REPLACEMENT_7f3a91c2';

export const register = (on: any) => {
  on('session.compact', async ($: any, event: any) => {
    $.ui.log(JSON.stringify({
      hook: 'session.compact',
      inputMessageCount: event.messages.length,
      replacementMarker: MARKER,
    }));
    return {
      messages: [{ role: 'assistant', text: MARKER, toolUses: [] }],
    };
  });
};
