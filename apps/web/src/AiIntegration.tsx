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
      <>On the command line, run <code>claude mcp add --transport http impressipes {'<address>'}</code> instead.</>,
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
      <>Grok chat apps only accept connectors on plans that offer them. If yours does, add a custom MCP connector with the address above.</>,
      <>With the xAI API, add a remote MCP tool with <code>server_url</code> set to the address.</>,
      <>Paste the skill's text into your custom instructions.</>,
    ],
  },
  {
    name: 'Grok bot',
    steps: [
      <>Point the bot's MCP client at the connector address above.</>,
      <>If the bot can't run the browser approval step, create a personal token (see below) and send it as <code>Authorization: Bearer imp_…</code>.</>,
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
        <p className="scribble md:text-[26px]">let a bot do the typing</p>
        <h1 className="text-[34px] leading-none md:text-[46px]">AI integration</h1>
        <p className="mt-2 text-ink-muted">
          Connect an AI assistant so it can save recipes from photos, plan meals and fill your shopping lists.
        </p>
      </div>

      <section className="card relative mb-6 flex flex-col gap-3 p-5">
        <h2 className="text-2xl">1. Connector address</h2>
        <p className="text-ink-muted">Every assistant below needs this address. It is the same for all of them.</p>
        <div className="flex flex-col gap-2 sm:flex-row">
          <input readOnly className="field min-w-0 flex-1" value={url} aria-label="Connector address" onFocus={(e) => e.target.select()} />
          <Button variant="sticker" className="self-start sm:self-auto" onClick={copy}><Copy />Copy</Button>
        </div>
        <p className="text-sm text-ink-muted">
          You'll be asked to approve access in this browser, so stay signed in. Changes the assistant makes show in the
          activity feed under its name and can be undone.
        </p>
      </section>

      <section className="card relative mb-6 flex flex-col gap-3 p-5">
        <h2 className="text-2xl">2. The skill</h2>
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

      <h2 className="mb-3 text-2xl">3. Set up your assistant</h2>
      <ul className="m-0 mb-6 flex list-none flex-col gap-4 p-0">
        {clients.map((c) => (
          <li key={c.name} className="card relative p-5">
            <h3 className="mb-2 text-xl">{c.name}</h3>
            <ol className="m-0 flex list-none flex-col gap-2 p-0">
              {c.steps.map((s, i) => (
                <li key={i} className="flex gap-2.5 [overflow-wrap:anywhere]">
                  <Check className="mt-1 size-4 flex-none text-accent" aria-hidden />
                  <span className="min-w-0">{s}</span>
                </li>
              ))}
            </ol>
            {c.note && <p className="mt-3 text-sm text-ink-muted">{c.note}</p>}
          </li>
        ))}
      </ul>

      <section className="card relative flex flex-col gap-2 p-5">
        <h2 className="text-2xl">No browser approval? Use a token</h2>
        <p className="text-ink-muted [overflow-wrap:anywhere]">
          Tools that can't open a browser can use a personal token instead. Whoever runs the server mints one with{' '}
          <code>go run ./cmd/token -member "Name" -name "Bot name"</code>. It is shown once. Send it as{' '}
          <code>Authorization: Bearer imp_…</code> and keep it secret.
        </p>
      </section>
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}
