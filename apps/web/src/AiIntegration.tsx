import { Check, ClipboardCopy, Copy, Download } from 'lucide-react';
import { useState } from 'react';
import { Button } from './ui/button';
import { Blob } from './ui/sticker';
import { Toast } from './ui/toast';

/**
 * Agents run commands and can set themselves up; chat apps can't add a connector
 * themselves, so their prompt asks them to walk the person through it instead.
 */
type Client = {
  name: string;
  kind: 'agent' | 'chat';
  where: string;
  prompt: (origin: string) => string;
  steps: React.ReactNode[];
  note?: string;
};

const check = 'Finally, call the list_tags tool and tell me what you can see, so I know it works.';

const agentPrompt = (origin: string) => `Please connect yourself to my family's Impressipes recipe box, so you can save recipes and manage our shopping list for me. Do each step yourself; only stop when you need me to click something.

1. Add a remote MCP server named "impressipes" at ${origin}/mcp (Streamable HTTP, OAuth sign-in). When it needs approval, give me the link: I'll approve it in my browser, where I'm signed in to Impressipes. If you can't do an OAuth sign-in, tell me and I'll ask the Impressipes admin for a personal token, which you send as the header "Authorization: Bearer <token>".
2. Install the Impressipes skill: download ${origin}/skill.md and save it as a skill named "impressipes" wherever you keep skills (for example skills/impressipes/SKILL.md). If you don't support skills, add its text to your standing instructions instead. A zipped copy is at ${origin}/skill.
3. ${check}`;

const chatPrompt = (where: string, skill: string) => (origin: string) => `I want to connect my family's Impressipes recipe box to you, and I'm new to this. Please walk me through it one small step at a time, and wait for me to say "done" before the next step.

- It's a custom connector (a remote MCP server) at this address: ${origin}/mcp
- ${where}
- It signs in with OAuth: once I add it, Impressipes opens a page where I approve access.
- ${skill}

Once it's connected, read ${origin}/skill.md and follow those instructions whenever I ask about recipes, meal plans or the shopping list. ${check}`;

const clients: Client[] = [
  {
    name: 'Claude',
    kind: 'chat',
    where: 'Paste it into a new chat on claude.ai or the Claude app.',
    prompt: chatPrompt(
      'In Claude it is under Settings → Connectors → Add custom connector.',
      'Then help me add the skill: I download the zip from the Impressipes address with /skill on the end and upload it under Settings → Capabilities → Skills.',
    ),
    steps: [
      <>Open Claude → <b>Settings → Connectors → Add custom connector</b>.</>,
      <>Paste the connector address and save. Claude opens an Impressipes page to approve access.</>,
      <>Download the skill and add it under <b>Settings → Capabilities → Skills</b>.</>,
    ],
  },
  {
    name: 'Claude Code',
    kind: 'agent',
    where: 'Paste it into Claude Code in any folder.',
    prompt: (origin) => `Set up my family's Impressipes recipe box for you to use:

1. Run: claude mcp add --transport http --scope user impressipes ${origin}/mcp
2. Install the skill: mkdir -p ~/.claude/skills/impressipes && curl -fsSL ${origin}/skill.md -o ~/.claude/skills/impressipes/SKILL.md
3. Tell me to run /mcp, pick impressipes and approve access in the browser (restart Claude Code first if it isn't listed). ${check}`,
    steps: [
      <>Run <code>claude mcp add --transport http --scope user impressipes {'<address>'}</code>.</>,
      <>Unzip the skill into <code>~/.claude/skills/</code>.</>,
      <>Run <code>/mcp</code>, pick impressipes and approve access.</>,
    ],
  },
  {
    name: 'ChatGPT',
    kind: 'chat',
    where: 'Paste it into a new ChatGPT chat.',
    prompt: chatPrompt(
      'In ChatGPT I first turn on Developer mode (Settings → Apps → Advanced settings), then create an app with the address and choose OAuth.',
      'ChatGPT has no skills, so help me paste the skill text into the app’s instructions.',
    ),
    steps: [
      <>Turn on <b>Developer mode</b>: Settings → Apps → Advanced settings.</>,
      <>Create an app (custom connector), paste the connector address, and choose <b>OAuth</b>.</>,
      <>Approve access on the Impressipes page. Paste the skill's text into the app's instructions.</>,
    ],
  },
  {
    name: 'Grok',
    kind: 'chat',
    where: 'Paste it into a new Grok chat.',
    prompt: chatPrompt(
      'In Grok, help me find where my plan lets me add a custom MCP connector.',
      'Then help me paste the skill text into my custom instructions.',
    ),
    steps: [
      <>Check that your Grok plan offers connectors, then add a custom MCP connector with the connector address.</>,
      <>Paste the skill's text into your custom instructions.</>,
    ],
  },
  {
    name: 'Grok bot',
    kind: 'agent',
    where: 'Send it to the bot as a message.',
    prompt: agentPrompt,
    steps: [
      <>Point the bot's MCP client at the connector address.</>,
      <>If the bot can't open a browser to approve access, ask whoever runs Impressipes for a personal token and send it as <code>Authorization: Bearer imp_…</code>.</>,
      <>Add the skill's text to the bot's system prompt.</>,
    ],
  },
  {
    name: 'Muse',
    kind: 'agent',
    where: 'Send it to Muse as a message.',
    prompt: agentPrompt,
    steps: [
      <>Look for a place to add an <b>MCP server</b> or <b>custom connector</b> in Muse's settings.</>,
      <>Paste the connector address. Choose OAuth if asked, or a bearer token if it only offers that.</>,
      <>Paste the skill's text into Muse's instructions.</>,
    ],
    note: 'We haven’t tested Muse’s settings screens, so the menu names may differ.',
  },
  {
    name: 'Something else',
    kind: 'agent',
    where: 'Paste it into your assistant. If it can’t set itself up, it will tell you what to click.',
    prompt: agentPrompt,
    steps: [
      <>Add a custom connector or remote MCP server with the connector address, using OAuth.</>,
      <>Add the skill's text to the assistant's instructions.</>,
    ],
  },
];

export function AiIntegration() {
  const [toast, setToast] = useState('');
  const [picked, setPicked] = useState<string | null>(null);
  const client = clients.find((c) => c.name === picked);
  const origin = window.location.origin;
  const url = `${origin}/mcp`;

  async function copy(text: string, what: string) {
    try {
      await navigator.clipboard.writeText(text);
      setToast(`${what} copied`);
    } catch {
      setToast(`Select the ${what.toLowerCase()} and copy it`);
    }
    setTimeout(() => setToast(''), 2500);
  }

  return (
    <div className="relative isolate max-w-180">
      <Blob shape="blob-b" className="-top-24 -right-20 size-64 bg-lilac" />
      <div className="relative mb-6">
        <p className="scribble md:text-[26px]">bring your AI to build your recipes</p>
        <h1 className="text-[34px] leading-none md:text-[46px]">AI integration</h1>
        <p className="mt-2 max-w-[34rem] text-ink">
          An AI assistant is a chat app, like Claude or ChatGPT, that you talk to in plain words. Connect one to
          Impressipes and it can work in your recipe box for you.
        </p>
      </div>

      <section className="card relative mb-6 flex flex-col gap-3 p-5">
        <h2 className="text-2xl">What can it do?</h2>
        <ul className="m-0 flex list-none flex-col gap-2 p-0">
          {[
            <>Send it a photo of a recipe card and say “save this”. It types the recipe in for you.</>,
            <>Ask “what should we eat this week?” and it plans meals from the recipes you already have.</>,
            <>Ask it to add what you need for those meals to your shopping list.</>,
          ].map((s, i) => (
            <li key={i} className="flex gap-2.5">
              <Check className="mt-1 size-4 flex-none text-accent" aria-hidden />
              <span className="min-w-0">{s}</span>
            </li>
          ))}
        </ul>
        <p className="text-sm text-ink-muted">
          It only does what you ask. Every change shows in the activity feed under its name, and you can undo it.
        </p>
      </section>

      <section className="card relative mb-6 flex flex-col gap-3 p-5">
        <h2 className="text-2xl">1. Pick your assistant</h2>
        <div className="flex flex-wrap gap-2" role="group" aria-label="Assistant">
          {clients.map((c) => (
            <Button
              key={c.name}
              size="sm"
              variant={c.name === picked ? 'primary' : 'flat'}
              aria-pressed={c.name === picked}
              onClick={() => setPicked(c.name)}
            >
              {c.name}
            </Button>
          ))}
        </div>
      </section>

      {client && (
        <section className="card relative mb-6 flex flex-col gap-3 p-5" aria-live="polite">
          <h2 className="text-2xl">2. Copy this, paste it into {client.name}</h2>
          <p className="text-ink-muted">
            {client.where}{' '}
            {client.kind === 'agent'
              ? 'It connects itself and installs the skill. You’ll only be asked to approve access.'
              : 'It walks you through each click, then learns how to look after your recipes.'}
          </p>
          <textarea
            readOnly
            className="field min-h-56 resize-y font-mono text-sm leading-relaxed"
            value={client.prompt(origin)}
            aria-label={`Setup prompt for ${client.name}`}
            onFocus={(e) => e.target.select()}
          />
          <Button variant="primary" className="self-start" onClick={() => copy(client.prompt(origin), 'Prompt')}>
            <ClipboardCopy />Copy prompt
          </Button>
          <p className="text-sm text-ink-muted">
            Stay signed in here: approving access happens in this browser. When it’s done, this step ticks off on your home page.
          </p>

          <details className="mt-2 rounded-md border-2 border-line p-4">
            <summary className="cursor-pointer font-bold">Prefer to do it by hand?</summary>
            <ol className="mt-3 mb-0 flex list-none flex-col gap-2 p-0">
              {client.steps.map((s, i) => (
                <li key={i} className="flex gap-2.5 [overflow-wrap:anywhere]">
                  <Check className="mt-1 size-4 flex-none text-accent" aria-hidden />
                  <span className="min-w-0">{s}</span>
                </li>
              ))}
            </ol>
            {client.note && <p className="mt-3 text-sm text-ink-muted">{client.note}</p>}
            <div className="mt-4 flex flex-col gap-2 sm:flex-row">
              <input readOnly className="field min-w-0 flex-1" value={url} aria-label="Connector address" onFocus={(e) => e.target.select()} />
              <Button variant="sticker" className="self-start sm:self-auto" onClick={() => copy(url, 'Connector address')}><Copy />Copy</Button>
            </div>
            <Button asChild variant="flat" size="sm" className="mt-3">
              <a href="/skill" download="skill.zip"><Download />Download skill</a>
            </Button>
            <p className="mt-2 text-sm text-ink-muted">
              The skill tells the assistant how to use your recipe box well: read photos carefully, check for duplicates,
              reuse your tags. Claude takes the zip as is; for others, paste <code>SKILL.md</code> into their instructions.
            </p>
          </details>
        </section>
      )}
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}
