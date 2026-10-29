import { Check, Copy, Download } from 'lucide-react';
import { useState } from 'react';
import { Button } from './ui/button';
import { Blob } from './ui/sticker';
import { Toast } from './ui/toast';

const mcpUrl = () => `${window.location.origin}/mcp`;

type Client = { name: string; steps: React.ReactNode[]; note?: string };

const clients: Client[] = [
  {
    name: 'Claude',
    steps: [
      <>Open Claude → <b>Settings → Connectors → Add custom connector</b>.</>,
      <>Paste the connector address above and save. Claude opens an Impressipes page to approve access.</>,
      <>Download the skill below and add it under <b>Settings → Capabilities → Skills</b>.</>,
      <>Using Claude Code? Run <code>claude mcp add --transport http impressipes {'<address>'}</code> instead.</>,
    ],
  },
  {
    name: 'ChatGPT / OpenAI',
    steps: [
      <>Turn on <b>Developer mode</b> in ChatGPT: Settings → Apps → Advanced settings.</>,
      <>Create an app (custom connector), paste the connector address, and choose <b>OAuth</b> for authentication.</>,
      <>Approve access on the Impressipes page. Paste the skill's text into the app's instructions.</>,
      <>With the API, add it as a remote MCP tool with <code>server_url</code> set to the address.</>,
    ],
  },
  {
    name: 'Grok',
    steps: [
      <>Check that your Grok plan offers connectors, then add a custom MCP connector with the connector address.</>,
      <>With the xAI API, add a remote MCP tool with <code>server_url</code> set to the address.</>,
      <>Paste the skill's text into your custom instructions.</>,
    ],
  },
  {
    name: 'Grok bot',
    steps: [
      <>Point the bot's MCP client at the connector address above.</>,
      <>If the bot can't open a browser to approve access, ask whoever runs Impressipes for a personal token and send it as <code>Authorization: Bearer imp_…</code>.</>,
      <>Add the skill's text to the bot's system prompt.</>,
    ],
  },
  {
    name: 'Muse',
    steps: [
      <>Look for a place to add an <b>MCP server</b> or <b>custom connector</b> in Muse's settings.</>,
      <>Paste the connector address above. Choose OAuth if asked, or a bearer token if it only offers that.</>,
      <>Paste the skill's text into Muse's instructions.</>,
    ],
    note: 'Muse has no connector screen we have tested, so check its own docs for the exact menu names.',
  },
];

export function AiIntegration() {
  const [toast, setToast] = useState('');
  const [picked, setPicked] = useState<string | null>(null);
  const client = clients.find((c) => c.name === picked);
  const url = mcpUrl();

  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      setToast('Connector address copied');
    } catch {
      setToast('Select the address and copy it');
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
        <h2 className="text-2xl">1. Copy the connector address</h2>
        <p className="text-ink-muted">Every assistant below needs this address. It is the same for all of them.</p>
        <div className="flex flex-col gap-2 sm:flex-row">
          <input readOnly className="field min-w-0 flex-1" value={url} aria-label="Connector address" onFocus={(e) => e.target.select()} />
          <Button variant="sticker" className="self-start sm:self-auto" onClick={copy}><Copy />Copy</Button>
        </div>
        <p className="text-sm text-ink-muted">
          You'll be asked to approve access in this browser, so stay signed in.
        </p>
      </section>

      <section className="card relative mb-6 flex flex-col gap-3 p-5">
        <h2 className="text-2xl">2. Download the skill</h2>
        <p className="text-ink-muted">
          The connector gives an assistant the tools. The skill tells it how to use them well: read a recipe photo
          carefully, check for duplicates first, reuse your tags, and split ingredients into amounts and units. Without
          it the assistant still works, just with less care.
        </p>
        <Button asChild variant="primary" className="self-start">
          <a href="/skill" download="skill.zip"><Download />Download skill</a>
        </Button>
        <p className="text-sm text-ink-muted">
          Claude takes the zip as is. For other assistants, unzip it and paste <code>SKILL.md</code> into their custom instructions.
        </p>
      </section>

      <section className="card relative flex flex-col gap-3 p-5">
        <h2 className="text-2xl">3. Set up your assistant</h2>
        <p className="text-ink-muted">Which assistant do you use?</p>
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
        {client && (
          <div className="mt-2">
            <h3 className="mb-2 text-xl">{client.name}</h3>
            <ol className="m-0 flex list-none flex-col gap-2 p-0">
              {client.steps.map((s, i) => (
                <li key={i} className="flex gap-2.5 [overflow-wrap:anywhere]">
                  <Check className="mt-1 size-4 flex-none text-accent" aria-hidden />
                  <span className="min-w-0">{s}</span>
                </li>
              ))}
            </ol>
            {client.note && <p className="mt-3 text-sm text-ink-muted">{client.note}</p>}
          </div>
        )}
      </section>
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}
