# Enable — Owner Restores a Disabled Rule

Restore a rule the owner suspended with `/disable`.

Triggers: only on explicit `/enable` typed by the user. Never invoke it yourself.

## Input

Prose or `<class> <item>`, matched against the files in `.harnez/state/disable/`: `/enable quota-1`, `/enable sprint skills again`. `/enable` with no argument restores all.

If the input matches no recorded item, list the recorded ones and change nothing. If it matches several and the user did not say "all", list them and ask.

## Steps

1. Delete the matching `.harnez/state/disable/<class>-<item>.yaml` files.
2. Say one line per item: `enabled <class> <item>`.
3. Enforce those rules again from now on.
