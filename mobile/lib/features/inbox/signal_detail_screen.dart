import 'package:flutter/material.dart';

/// Signal detail screen placeholder - will be fully implemented in Task 2.
class SignalDetailScreen extends StatelessWidget {
  final String signalId;

  const SignalDetailScreen({super.key, required this.signalId});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Signal')),
      body: Center(child: Text('Signal: $signalId')),
    );
  }
}
