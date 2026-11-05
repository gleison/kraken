"""K-01: verify the approved TUI library's public headless testing API.

This fixture has no Kraken runtime, model, repository access or execution controls.
"""
import unittest

from textual.app import App, ComposeResult
from textual.widgets import Button, DataTable, Static


class FixtureApp(App):
    def compose(self) -> ComposeResult:
        yield Static('Aguardando aprovação', id='status')
        yield DataTable(id='tasks')
        yield Button('Aprovar plano', id='approve')

    def on_mount(self):
        table = self.query_one(DataTable)
        table.add_columns('Tarefa', 'Estado')
        table.add_row('TASK-1', 'aguardando_aprovacao')

    def on_button_pressed(self, event: Button.Pressed):
        if event.button.id == 'approve':
            self.query_one('#status', Static).update('Ação explícita recebida')


class TextualSpike(unittest.IsolatedAsyncioTestCase):
    async def test_explicit_action_and_small_terminal(self):
        app = FixtureApp()
        async with app.run_test(size=(60, 20)) as pilot:
            self.assertEqual(app.query_one(DataTable).row_count, 1)
            await pilot.click('#approve')
            self.assertIn('Ação explícita recebida', str(app.query_one('#status', Static).content))
            await pilot.resize_terminal(40, 15)
            await pilot.pause()
            self.assertEqual(app.size.width, 40)

    async def test_quit_does_not_create_approval(self):
        app = FixtureApp()
        async with app.run_test() as pilot:
            await pilot.pause()
            self.assertIn('Aguardando aprovação', str(app.query_one('#status', Static).content))


if __name__ == '__main__':
    unittest.main()
