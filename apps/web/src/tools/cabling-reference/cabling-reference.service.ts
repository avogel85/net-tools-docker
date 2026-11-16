export type Row = Record<string, string>

/** Twisted-pair categories (ANSI/TIA-568 and ISO/IEC 11801). */
export const COPPER: Row[] = [
  { name: 'Cat5e', bandwidth: '100 MHz', speeds: '1 Gbit/s 100 m · 2.5 Gbit/s 100 m (NBASE-T) · 5 Gbit/s ~100 m (best effort)', notes: 'Minimum for new 1G installs; fine for most PoE' },
  { name: 'Cat6', bandwidth: '250 MHz', speeds: '1–5 Gbit/s 100 m · 10 Gbit/s 37–55 m', notes: '10G over short runs only, alien crosstalk limits longer bundles' },
  { name: 'Cat6A', bandwidth: '500 MHz', speeds: '10 Gbit/s 100 m', notes: 'Recommended for new installs, Wi-Fi 6/7 APs and PoE++ (90 W)' },
  { name: 'Cat7 / 7A (ISO class F/FA)', bandwidth: '600 / 1000 MHz', speeds: '10 Gbit/s 100 m', notes: 'Not a TIA standard; needs GG45/TERA connectors for its full rating' },
  { name: 'Cat8.1 / 8.2', bandwidth: '2000 MHz', speeds: '25/40 Gbit/s 30 m (25GBASE-T, 40GBASE-T)', notes: 'Data-centre top-of-rack links only' },
]

/** Fiber types (TIA-492 / IEC 60793) with jacket colour and modal bandwidth. */
export const FIBER: Row[] = [
  { name: 'OM1', core: '62.5/125 µm multimode', colour: 'orange', bandwidth: '200 MHz·km @ 850 nm', notes: 'Legacy; 1G up to ~275 m, 10G only ~33 m' },
  { name: 'OM2', core: '50/125 µm multimode', colour: 'orange', bandwidth: '500 MHz·km @ 850 nm', notes: 'Legacy; 10G-SR 82 m' },
  { name: 'OM3', core: '50/125 µm laser-optimised MM', colour: 'aqua', bandwidth: '2000 MHz·km @ 850 nm', notes: '10G-SR 300 m, 25G-SR 70 m, 100G-SR4 70 m' },
  { name: 'OM4', core: '50/125 µm laser-optimised MM', colour: 'aqua or violet (heather)', bandwidth: '4700 MHz·km @ 850 nm', notes: '10G-SR 400 m, 25G-SR 100 m, 100G-SR4 100 m' },
  { name: 'OM5', core: '50/125 µm wideband MM', colour: 'lime green', bandwidth: '4700 MHz·km @ 850 nm, 2470 @ 953 nm', notes: 'For SWDM (850–950 nm); same reach as OM4 otherwise' },
  { name: 'OS2', core: '9/125 µm single-mode', colour: 'yellow', bandwidth: '— (single-mode)', notes: 'Low-water-peak; 0.4 dB/km @ 1310, 0.3 dB/km @ 1550 nm (cabled); kilometres' },
]

/** Common optics: form factor, wavelength, fiber, connector and reach. */
export const OPTICS: Row[] = [
  { standard: '1000BASE-SX', form: 'SFP', wavelength: '850 nm', fiber: 'MM', connector: 'LC duplex', reach: '550 m OM2+' },
  { standard: '1000BASE-LX', form: 'SFP', wavelength: '1310 nm', fiber: 'SM', connector: 'LC duplex', reach: '10 km' },
  { standard: '1000BASE-T', form: 'SFP (copper)', wavelength: '—', fiber: 'Cat5e+', connector: 'RJ45', reach: '100 m' },
  { standard: '1000BASE-BX', form: 'SFP', wavelength: '1310/1490 nm', fiber: 'SM single strand', connector: 'LC simplex', reach: '10–40 km (pair: U + D)' },
  { standard: '10GBASE-SR', form: 'SFP+', wavelength: '850 nm', fiber: 'MM', connector: 'LC duplex', reach: '300 m OM3 · 400 m OM4' },
  { standard: '10GBASE-LR', form: 'SFP+', wavelength: '1310 nm', fiber: 'SM', connector: 'LC duplex', reach: '10 km' },
  { standard: '10GBASE-ER', form: 'SFP+', wavelength: '1550 nm', fiber: 'SM', connector: 'LC duplex', reach: '40 km' },
  { standard: '10GBASE-ZR', form: 'SFP+', wavelength: '1550 nm', fiber: 'SM', connector: 'LC duplex', reach: '80 km (not an IEEE standard)' },
  { standard: '10GBASE-T', form: 'SFP+ (copper)', wavelength: '—', fiber: 'Cat6A', connector: 'RJ45', reach: '30–80 m (module power limits)' },
  { standard: '10G DAC', form: 'SFP+', wavelength: '—', fiber: 'twinax', connector: 'fixed', reach: '1–7 m (passive ≤5 m)' },
  { standard: '25GBASE-SR', form: 'SFP28', wavelength: '850 nm', fiber: 'MM', connector: 'LC duplex', reach: '70 m OM3 · 100 m OM4' },
  { standard: '25GBASE-LR', form: 'SFP28', wavelength: '1310 nm', fiber: 'SM', connector: 'LC duplex', reach: '10 km' },
  { standard: '40GBASE-SR4', form: 'QSFP+', wavelength: '850 nm (4 lanes)', fiber: 'MM', connector: 'MPO-12', reach: '100 m OM3 · 150 m OM4' },
  { standard: '40GBASE-LR4', form: 'QSFP+', wavelength: '1310 nm CWDM (4 λ)', fiber: 'SM', connector: 'LC duplex', reach: '10 km' },
  { standard: '100GBASE-SR4', form: 'QSFP28', wavelength: '850 nm (4 lanes)', fiber: 'MM', connector: 'MPO-12', reach: '70 m OM3 · 100 m OM4' },
  { standard: '100GBASE-LR4', form: 'QSFP28', wavelength: '1310 nm LAN-WDM (4 λ)', fiber: 'SM', connector: 'LC duplex', reach: '10 km' },
  { standard: '100G-CWDM4', form: 'QSFP28', wavelength: '1271–1331 nm (4 λ)', fiber: 'SM', connector: 'LC duplex', reach: '2 km (MSA)' },
  { standard: '100GBASE-DR', form: 'QSFP28', wavelength: '1310 nm (1 λ, PAM4)', fiber: 'SM', connector: 'LC duplex', reach: '500 m' },
  { standard: '400GBASE-DR4', form: 'QSFP-DD / OSFP', wavelength: '1310 nm (4 lanes)', fiber: 'SM', connector: 'MPO-12 APC', reach: '500 m' },
  { standard: '400GBASE-FR4', form: 'QSFP-DD / OSFP', wavelength: '1271–1331 nm (4 λ)', fiber: 'SM', connector: 'LC duplex', reach: '2 km' },
]

export const CONNECTORS: Row[] = [
  { name: 'LC', type: 'Small form factor, 1.25 mm ferrule', use: 'SFP/SFP+/SFP28/QSFP (duplex) — the default today' },
  { name: 'SC', type: '2.5 mm ferrule, push-pull', use: 'Older patch panels, FTTH (often SC/APC green)' },
  { name: 'ST', type: 'Bayonet, 2.5 mm ferrule', use: 'Legacy multimode installs' },
  { name: 'MPO / MTP (12 or 16 fibres)', type: 'Multi-fibre push-on', use: 'Parallel optics (SR4, DR4) and trunk cables; mind polarity type A/B/C' },
  { name: 'UPC (blue) vs APC (green)', type: 'Flat vs 8° angled polish', use: 'Never mate UPC with APC — high loss and damage; APC is common for PON/FTTH and 400G DR4' },
]

export function filterRows(rows: Row[], q: string): Row[] {
  const s = q.trim().toLowerCase()
  return s ? rows.filter(r => Object.values(r).some(v => v.toLowerCase().includes(s))) : rows
}
