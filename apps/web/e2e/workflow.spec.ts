import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const here = path.dirname(fileURLToPath(import.meta.url))
const fixture = (name: string) => path.resolve(here, '../../../testdata', name)

// Login -> create project -> upload v1 -> upload v2 -> run analysis ->
// view the compatibility report -> evaluate a release policy.
test('complete compatibility workflow', async ({ page }) => {
  page.on('dialog', (dialog) => dialog.accept())
  const email = `e2e-${Date.now()}@example.com`

  await page.goto('/login')
  await page.getByRole('button', { name: 'Need an account? Register' }).click()
  await page.getByLabel('Organization').fill('E2E Org')
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill('correct-horse-1')
  await page.getByRole('button', { name: 'Create account' }).click()

  await page.getByRole('link', { name: 'Projects' }).click()
  await page.getByLabel('Project name').fill('Users API')
  await page.getByRole('button', { name: 'Create project' }).click()
  await page.getByRole('link', { name: 'Users API' }).click()

  for (const [label, file] of [['v1.0.0', 'compatible/users-v1.yaml'], ['v2.0.0', 'breaking/users-v2-breaking.yaml']] as const) {
    await page.getByLabel('Version label').fill(label)
    await page.getByLabel('OpenAPI file').setInputFiles(fixture(file))
    await page.getByRole('button', { name: 'Upload version' }).click()
    await expect(page.getByText(`Stored ${label}`)).toBeVisible()
  }

  await page.getByRole('tab', { name: 'Compatibility' }).click()
  await page.getByLabel('Base version').selectOption({ label: 'v1.0.0' })
  await page.getByLabel('Target version').selectOption({ label: 'v2.0.0' })
  await page.getByRole('button', { name: 'Run analysis' }).click()
  await page.getByRole('link', { name: 'View' }).click()
  await expect(page.getByRole('heading', { name: 'Analysis' })).toBeVisible()
  await expect(page.locator('td span', { hasText: 'BREAKING' }).first()).toBeVisible()

  await page.getByRole('link', { name: '← Back to project' }).click()
  await page.getByRole('tab', { name: 'Policies' }).click()
  await page.getByLabel('Policy name').fill('block-breaking')
  await page.getByLabel('BREAKING', { exact: true }).check()
  await page.getByRole('button', { name: 'Create policy' }).click()
  await expect(page.locator('li', { hasText: 'block-breaking' })).toBeVisible()

  await page.getByLabel('Analysis', { exact: true }).selectOption({ index: 1 })
  await page.getByLabel('Policy', { exact: true }).selectOption({ index: 1 })
  await page.getByRole('button', { name: 'Evaluate release' }).click()
  await expect(page.getByText('Decision:')).toBeVisible()
  await expect(page.getByRole('status').getByText('BLOCK')).toBeVisible()
})
