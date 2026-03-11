import 'package:flutter/material.dart';

/// Inbox screen placeholder - will be fully implemented in Task 2.
class InboxScreen extends StatelessWidget {
  const InboxScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Inbox')),
      body: const Center(child: Text('Loading inbox...')),
    );
  }
}
