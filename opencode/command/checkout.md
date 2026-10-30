---
description: clone <owner/repo> into the workspace and start working on it
---
Create a fresh checkout of the GitHub repository named below and work in it.

1. Run the bundled helper to create the checkout:

   `opencode-checkout "$ARGUMENTS"`

   The helper clones the repository over HTTPS into the workspace at
   `/workspace/github/<owner>/<repo>` (or `$OPENCODE_CHECKOUT_ROOT` when set)
   and prints the absolute path of the checkout. Private repositories work
   when Git credentials are configured for the container, for example through
   `gh auth setup-git` or a GitHub App credential helper. When the checkout
   already exists the helper refreshes it instead of cloning it again.

2. Read the files in the printed path and summarise what the project is.

3. Continue the rest of the session in that directory.

If no repository was supplied, ask which GitHub repository to check out and wait
for the answer before running anything.
